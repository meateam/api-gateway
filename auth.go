package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

//  authRequired searches for the authorization header to check if the client has a jwt token.
//	If the token is not valid or expired, it will redirect the client to the auth service.
//	If the token is valid, it will inject user to the gin context.
func authRequired(c *gin.Context) {
	auth, err := c.Cookie("kd-token")
	if auth == "" || err != nil {
		authArr := strings.Fields(c.GetHeader("Authorization"))
		if len(authArr) < 2 {
			redirectToAuthService(c)
			return
		}

		if authArr[0] != "Bearer" {
			redirectToAuthService(c)
			return
		}

		auth = authArr[1]
	}

	if auth == "" {
		redirectToAuthService(c)
		return
	}

	token, err := jwt.Parse(auth, func(token *jwt.Token) (interface{}, error) {
		// Validates the alg is what we expect:
		secret := viper.GetString(configSecret)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			redirectToAuthService(c)
			logger.Infof("unexpected signing method: %v", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})

	if err != nil {
		logger.Infof("error while parsing the token")
		redirectToAuthService(c)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok || !token.Valid {
		logger.Infof("the token is not valid")
		redirectToAuthService(c)
		return
	}

	// Check type assertion
	id, idOk := claims["id"].(string)
	firstName, firstNameOk := claims["firstName"].(string)
	lastName, lastNameOk := claims["lastName"].(string)

	// If any of the claims are invalid then redirect to authentication
	if !idOk || !firstNameOk || !lastNameOk {
		logger.Infof("the token's claims are invalid")
		redirectToAuthService(c)
		return
	}

	// Check type assertion.
	// For some reason can't convert directly to int64
	exp, ok := claims["exp"].(float64)
	if !ok {
		logger.Infof("token's exp: %v not valid", claims["exp"])
		redirectToAuthService(c)
		return
	}

	expTime := time.Unix(int64(exp), 0)
	timeRemaining := expTime.Sub(time.Now())

	if timeRemaining <= 0 {
		logger.Infof("token has expired at %v . The user is %s", expTime, id)
		redirectToAuthService(c)
		return
	}

	c.Set("User", user{
		id:        id,
		firstName: firstName,
		lastName:  lastName,
	})

	c.Next()
}

func redirectToAuthService(c *gin.Context) {
	authURL := viper.GetString(configAuthURL)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
	c.Abort()
}
