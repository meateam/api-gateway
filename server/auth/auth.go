package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
)

const (
	// AuthCookie is the name of the authorization cookie.
	AuthCookie = "kd-token"

	// AuthHeader is the key of the authorization header.
	AuthHeader = "Authorization"

	// AuthHeaderBearer is the prefix for the authorization token in AuthHeader.
	AuthHeaderBearer = "Bearer"
)

// Middleware validates the jwt token in c.Cookie(AuthCookie) or c.GetHeader(AuthHeader).
// If the token is not valid or expired, it will redirect the client to authURL.
// If the token is valid, it will set the user's data into the gin context
// at user.ContextUserKey.
func Middleware(secret string, authURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth, err := c.Cookie(AuthCookie)
		if auth == "" || err != nil {
			authArr := strings.Fields(c.GetHeader(AuthHeader))
			if len(authArr) < 2 {
				redirectToAuthService(c, authURL)
				return
			}

			if authArr[0] != AuthHeaderBearer {
				redirectToAuthService(c, authURL)
				return
			}

			auth = authArr[1]
		}

		if auth == "" {
			redirectToAuthService(c, authURL)
			return
		}

		token, err := jwt.Parse(auth, func(token *jwt.Token) (interface{}, error) {
			// Validates the alg is what we expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				redirectToAuthService(c, authURL)
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(secret), nil
		})

		if err != nil {
			redirectToAuthService(c, authURL)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)

		if !ok || !token.Valid {
			redirectToAuthService(c, authURL)
			return
		}

		// Check type assertion.
		// For some reason can't convert directly to int64.
		iat, ok := claims["iat"].(float64)
		if !ok {
			redirectToAuthService(c, authURL)
			return
		}

		passed := time.Since(time.Unix(int64(iat), 0))

		// Token expired.
		if time.Hour*24 < passed {
			redirectToAuthService(c, authURL)
			return
		}

		// Check type assertion.
		id, idOk := claims["id"].(string)
		firstName, firstNameOk := claims["firstName"].(string)
		lastName, lastNameOk := claims["lastName"].(string)

		// If any of the claims are invalid then redirect to authentication.
		if !idOk || !firstNameOk || !lastNameOk {
			redirectToAuthService(c, authURL)
			return
		}

		c.Set(user.ContextUserKey, user.User{
			ID:        id,
			FirstName: firstName,
			LastName:  lastName,
		})

		c.Next()
	}
}

// redirectToAuthService temporary redirects c to authURL and aborts the pending handlers.
func redirectToAuthService(c *gin.Context, authURL string) {
	c.Redirect(http.StatusTemporaryRedirect, authURL)
	c.Abort()
}
