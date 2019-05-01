package main

import (
	"log"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"github.com/dgrijalva/jwt-go"
)

func setupRouter() (r *gin.Engine, close func()) {
	// Disable Console Color
	gin.DisableConsoleColor()
	r = gin.Default()

	// Default cors handeling.
	corsConfig := cors.DefaultConfig()
	corsConfig.AddExposeHeaders("x-uploadid")
	corsConfig.AddAllowHeaders("cache-control", "x-requested-with", "content-disposition", "content-range")
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	// Auth middleware
	r.Use(authRequired)

	// Initiate file router.
	fileConn, err := initServiceConn(viper.GetString(configfileService))
	if err != nil {
		log.Fatalf("couldn't setup file service connection: %v", err)
	}

	uploadConn, err := initServiceConn(viper.GetString(configUploadService))
	if err != nil {
		log.Fatalf("couldn't setup upload service connection: %v", err)
	}

	downloadConn, err := initServiceConn(viper.GetString(configDownloadService))
	if err != nil {
		log.Fatalf("couldn't setup download service connection: %v", err)
	}
	
	// Initiate file router.
	fr := &fileRouter{}

	// Initiate upload router.
	ur := &uploadRouter{}

	// Initiate client connection to file service.
	fr.setup(r, fileConn, downloadConn)

	// Initiate client connection to upload service.
	ur.setup(r, uploadConn, fileConn)

	// Creating a slice to manage connections
	conns := []*grpc.ClientConn{fileConn, uploadConn, downloadConn}

	// Defines a function that is closing all connections in order to defer it outside.
	close = func() {
		for _, v := range conns {
			v.Close()
		}

		return
	}

	return
}

func initServiceConn(url string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(url, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

//  authRequired searches for the authorization header to check if the client has a jwt token.
//	If the token is not valid or expired, it will redirect the client to the auth service.
//	If the token is valid, it will inject user to the gin context
func authRequired(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		redirectToAuthService(c)
		return
	}
	authArr := strings.Fields(auth)
	if len(authArr) < 2 {
		redirectToAuthService(c)
		return
	}
	_, tokenString := authArr[0], authArr[1]
	
	secret := viper.GetString(configSecret)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		
		// Validates the alg is what we expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			redirectToAuthService(c)
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		return []byte(secret), nil

	})
	if err != nil {
		redirectToAuthService(c)
		return
	} 
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {	
		iat_float, ok := claims["iat"].(float64)
		if !ok {
			redirectToAuthService(c)
			return
		}
		iat := int64(iat_float)
		passed := time.Since(time.Unix(iat, 0))
		
		// Token expired
		if time.Hour*24 < passed {
			redirectToAuthService(c)
			return
		}

		// Check type assertion
		c.Set("User", user{
			id: claims["id"].(string),
			firstName: claims["firstName"].(string),
			lastName: claims["lastName"].(string),
		})
		return
	} 
	redirectToAuthService(c)
	return

}

func redirectToAuthService(c *gin.Context) {
	authURL := viper.GetString(configAuthUrl)
	c.Redirect(http.StatusMovedPermanently, authURL)
	c.Abort()
	return
}
