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

	if authArr[0] != "Bearer" {
		redirectToAuthService(c)
		return
	}

	tokenString := authArr[1]

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validates the alg is what we expect:
		secret := viper.GetString(configSecret)
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
		// Check type assertion.
		// For some reason can't convert directly to int64
		iat, ok := claims["iat"].(float64)
		if !ok {
			redirectToAuthService(c)
			return
		}

		passed := time.Since(time.Unix(int64(iat), 0))

		// Token expired
		if time.Hour*24 < passed {
			redirectToAuthService(c)
			return
		}

		// Check type assertion
		id, idOk := claims["id"].(string)
		firstName, firstNameOk := claims["firstName"].(string)
		lastName, lastNameOk := claims["lastName"].(string)

		// If any of the claims are invalid then redirect to authentication
		if !idOk || !firstNameOk || !lastNameOk {
			redirectToAuthService(c)
			return
		}

		c.Set("User", user{
			id:        id,
			firstName: firstName,
			lastName:  lastName,
		})
	}
	c.Next()
}

func redirectToAuthService(c *gin.Context) {
	authURL := viper.GetString(configAuthURL)
	c.Redirect(http.StatusMovedPermanently, authURL)
	c.Abort()
}
