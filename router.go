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
	const numOfRPCConns = 3

	// Disable Console Color
	gin.DisableConsoleColor()
	r = gin.Default()

	// In a form upload - defines how many saved in RAM, the rest saved in /tmp.
	r.MaxMultipartMemory = 5 << 20

	// Default cors handeling.
	corsConfig := cors.DefaultConfig()
	corsConfig.AddAllowHeaders("cache-control", "x-requested-with", "content-disposition")
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	// Auth middleware
	r.Use(authRequired)

	// Initiate file router.
	fr := &fileRouter{
		fileServiceURL: viper.GetString(configfileService),
	}

	// Initiate upload router.
	ur := &uploadRouter{
		uploadServiceURL: viper.GetString(configUploadService),
	}

	// Initiate download router.
	dr := &downloadRouter{
		downloadServiceURL: viper.GetString(configDownloadService),
	}

	// Creating a slice to manage connections
	conns := make([]*grpc.ClientConn, 0, numOfRPCConns)

	// Initiate client connection to file service.
	// Appends The connection to the connections slice.
	fconn, err := fr.setup(r)
	if err != nil {
		log.Fatalf("couldn't setup upload router: %v", err)
	}
	conns = append(conns, fconn)

	// Initiate client connection to upload service.
	// Appends The connection to the connections slice.
	uconn, err := ur.setup(r, fconn)
	if err != nil {
		log.Fatalf("couldn't setup upload router: %v", err)
	}
	conns = append(conns, uconn)

	// Initiate client connection to download service.
	// Appends The connection to the connections slice.
	dconn, err := dr.setup(r, fconn)
	if err != nil {
		log.Fatalf("couldn't setup download router: %v", err)
	}
	conns = append(conns, dconn)

	// Defines a function that is closing all connections in order to defer it outside.
	close = func() {
		for _, v := range conns {
			v.Close()
		}
	}
	return
}

/*  the function search for the authrization header to check if the client has a jwt token.
	If the token is not valid or expired, it will redirect the client to the auth service
	If the token is valid, it will inject user to the gin context
*/
func authRequired(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		redirectToAuthService(c)
		return
	}
	authArr := strings.Fields(auth)
	_, tokenString := authArr[0], authArr[1]
	
	secret := viper.GetString(configSecret)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validates the alg is what we expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			redirectToAuthService(c)
		}
		
		return []byte(secret), nil
	})
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {	
		iat := int64(claims["iat"].(float64))
		passed := time.Since(time.Unix(iat, 0))
		if time.Hour*24 < passed {
			fmt.Println("Token Expired!")
			redirectToAuthService(c)
			return
		}
		c.Set("User", user{
			id: claims["id"].(string),
			firstName: claims["firstName"].(string),
			lastName: claims["lastName"].(string),
		})
		fmt.Println(extractRequestUser(c))
	} else {
		fmt.Println(err)
		redirectToAuthService(c)
		return
	}

}

func redirectToAuthService(c *gin.Context) {
	host := viper.GetString(configHost)
	c.Redirect(http.StatusMovedPermanently, host + "/auth/login")
	c.Abort()
}
