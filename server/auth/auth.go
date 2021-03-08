package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	// AuthCookie is the name of the authorization cookie.
	AuthCookie = "kd-token"

	// AuthHeader is the key of the authorization header.
	AuthHeader = "Authorization"

	// AuthHeaderBearer is the prefix for the authorization token in AuthHeader.
	AuthHeaderBearer = "Bearer"

	// FirstNameLabel is the claim name for the firstname of the user
	FirstNameLabel = "firstName"

	// LastNameLabel is the claim name for the lastname of the user
	LastNameLabel = "lastName"

	// DisplayNameLabel is the claim name of the display name of the user
	DisplayNameLabel = "displayName"

	// CurrentUnitLabel is the claim name of the current unit of the user
	CurrentUnitLabel = "currentUnit"

	// RankLabel is the claim name of the rank of the user
	RankLabel = "rank"

	// JobLabel is the claim name of the job of the user
	JobLabel = "job"

	// UserNameLabel is the label for the full user name.
	UserNameLabel = "username"

	// AuthTypeHeader is the key of the servive-host header
	AuthTypeHeader = "Auth-Type"

	// DocsAuthTypeValue is the value of the docs-service for AuthTypeHeader key
	DocsAuthTypeValue = "Docs"

	// DEPRECATED: ServiceAuthTypeValue is the value of service for AuthTypeHeader key
	ServiceAuthTypeValue = "Service"

	// ServiceAuthCodeTypeValue is the value of service using the authorization code flow for AuthTypeHeader key
	ServiceAuthCodeTypeValue = "Service AuthCode"

	// ConfigWebUI is the name of the environment variable containing the path to the ui.
	ConfigWebUI = "web_ui"

	// DriveClientName is the client name of the Drive UI client.
	DriveClientName = "DriveUI"
)

// Router is a structure that handels the authentication middleware.
type Router struct {
	logger *logrus.Logger
}

// Secrets is a struct that holds the application secrets.
type Secrets struct {
	Drive string
	Docs  string
}

// NewRouter creates a new Router. If logger is non-nil then it will be
// set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	return r
}

// Middleware check that the client has valid authentication to use the route
// This function also set variables like user and service to the context.
func (r *Router) Middleware(secrets Secrets, authURL string) gin.HandlerFunc {
	return func(c *gin.Context) {

		serviceName := c.GetHeader(AuthTypeHeader)

		if serviceName != oauth.DropboxAuthTypeValue && serviceName != oauth.CargoAuthTypeValue && serviceName != ServiceAuthCodeTypeValue {
			// If not an external service, then it is a user (from the main Drive UI client).
			oauth.SetApmClient(c, DriveClientName)
			secret := secrets.Drive

			if serviceName == DocsAuthTypeValue {
				oauth.SetApmClient(c, DocsAuthTypeValue)
				secret = secrets.Docs
			}

			r.UserMiddleware(c, secret, authURL)
		}
		c.Next()
	}
}

// UserMiddleware is a middleware which validates the user requesting the operation.
// It validates the jwt token in c.Cookie(AuthCookie) or c.GetHeader(AuthHeader).
// If the token is not valid or expired, it will redirect the client to authURL.
// If the token is valid, it will set the user's data into the gin context
// at user.ContextUserKey.
func (r *Router) UserMiddleware(c *gin.Context, secret string, authURL string) {
	token := r.ExtractToken(secret, authURL, c)
	// Check if the extraction was successful
	if token == nil {
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok || !token.Valid {
		r.redirectToAuthService(c, authURL, fmt.Sprintf("invalid token: %v", token))
		return
	}

	// Check type assertion
	id, idOk := claims["id"]
	firstName, firstNameOk := claims[FirstNameLabel]
	lastName, lastNameOk := claims[LastNameLabel]
	displayName := claims[DisplayNameLabel]
	job := claims[JobLabel]
	currentUnit := claims[CurrentUnitLabel]
	rank := claims[RankLabel]

	// If any of the claims are invalid then redirect to authentication
	if !idOk || !firstNameOk || !lastNameOk {
		r.redirectToAuthService(c, authURL, fmt.Sprintf("invalid token claims: %v", claims))
		return
	}

	// Check type assertion.
	// For some reason can't convert directly to int64
	exp, ok := claims["exp"].(float64)
	if !ok {
		r.redirectToAuthService(c, authURL, fmt.Sprintf("invalid token exp: %v", claims["exp"]))
		return
	}

	expTime := time.Unix(int64(exp), 0)
	timeUntilExp := time.Until(expTime)

	// Verify again that the token is not expired
	if timeUntilExp <= 0 {
		r.redirectToAuthService(c, authURL, fmt.Sprintf("user %s token expired at %s", expTime, id))
		return
	}

	authenticatedUser := user.User{
		ID:          fmt.Sprintf("%s", id),
		FirstName:   fmt.Sprintf("%s", firstName),
		LastName:    fmt.Sprintf("%s", lastName),
		Source:      user.InternalUserSource,
		DisplayName: fmt.Sprintf("%s", displayName),
		CurrentUnit: fmt.Sprintf("%s", currentUnit),
		Rank:        fmt.Sprintf("%s", rank),
		Job:         fmt.Sprintf("%s", job),
	}

	c.Set(user.ContextUserKey, authenticatedUser)

	user.SetApmUser(c, authenticatedUser)

	c.Set(oauth.ContextAppKey, oauth.DriveAppID)

	c.Set(oauth.ContextAppKey, oauth.DriveAppID)

	c.Next()
}

// ExtractToken extract the jwt token from c.Cookie(AuthCookie) or c.GetHeader(AuthHeader).
// If the token is invalid or expired, it will redirect the client to authURL, and return nil.
// If the token is valid, it will return the token.
func (r *Router) ExtractToken(secret string, authURL string, c *gin.Context) *jwt.Token {
	auth, err := c.Cookie(AuthCookie)

	// If there is no cookie check if a header exists
	if auth == "" || err != nil {
		authArr := strings.Fields(c.GetHeader(AuthHeader))

		// No authorization cookie/header sent
		if len(authArr) == 0 {
			r.redirectToAuthService(c, authURL, fmt.Sprintf("no authorization cookie/header sent"))
			return nil
		}

		// The header value missing the correct prefix
		if authArr[0] != AuthHeaderBearer {
			r.redirectToAuthService(c, authURL,
				fmt.Sprintf("authorization header is not legal. Value should start with 'Bearer': %v", authArr[0]))
			return nil
		}

		// The value of the header doesn't contain the token
		if len(authArr) < 2 {
			r.redirectToAuthService(c, authURL,
				fmt.Sprintf("no token sent in header %v", authArr))
			return nil
		}

		auth = authArr[1]
	}

	// The auth token is empty
	if auth == "" {
		r.redirectToAuthService(c, authURL, fmt.Sprintf("there is no auth token"))
		return nil
	}

	token, err := jwt.Parse(auth, func(token *jwt.Token) (interface{}, error) {
		// Validates the alg is what we expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			errMessage := fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			r.redirectToAuthService(c, authURL, errMessage.Error())
			return nil, errMessage
		}

		return []byte(secret), nil
	})

	// Could be an invalid jwt, a wrong signature, or a passed exp
	if err != nil {
		r.redirectToAuthService(c, authURL,
			fmt.Sprintf("error while parsing the JWT token. %v. token: %v", err, auth))
		return nil
	}

	return token
}

// redirectToAuthService temporary redirects c to authURL and aborts the pending handlers.
func (r *Router) redirectToAuthService(c *gin.Context, authURL string, reason string) {
	r.logger.Info(reason)
	redirectURI := viper.GetString(ConfigWebUI) + c.Request.RequestURI
	encodedRedirectURI := url.QueryEscape(redirectURI)
	authRedirectURL := fmt.Sprintf("%s?RelayState=%s", authURL, encodedRedirectURI)
	c.Redirect(http.StatusTemporaryRedirect, authRedirectURL)
	c.Abort()
}
