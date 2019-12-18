package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	dpb "github.com/meateam/delegation-service/proto/delegation-service"
	spb "github.com/meateam/spike-service/proto/spike-service"
	"github.com/sirupsen/logrus"
	"go.elastic.co/apm"
	"google.golang.org/grpc"
)

const (
	// AuthCookie is the name of the authorization cookie.
	AuthCookie = "kd-token"

	// AuthHeader is the key of the authorization header.
	AuthHeader = "Authorization"

	// AuthHeaderBearer is the prefix for the authorization token in AuthHeader.
	AuthHeaderBearer = "Bearer"

	// AuthUserHeader is the key of the header which indicates whether an action is made on behalf of a user
	AuthUserHeader = "AuthUser"

	// FirstNameClaim is the claim name for the firstname of the user
	FirstNameClaim = "firstName"

	// LastNameClaim is the claim name for the lastname of the user
	LastNameClaim = "lastName"

	// UserNameLabel is the label for the full user name.
	UserNameLabel = "username"

	// ServiceHostHeader is the key of the servive-host header
	ServiceHostHeader = "ServiceHost"
)

// Router is a structure that handels the authentication middleware.
type Router struct {
	spikeClient    spb.SpikeClient
	delegateClient dpb.DelegationClient
	logger         *logrus.Logger
}

// NewRouter creates a new Router. If logger is non-nil then it will be
// set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	spikeConn *grpc.ClientConn,
	delegateConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.spikeClient = spb.NewSpikeClient(spikeConn)

	r.delegateClient = dpb.NewDelegationClient(delegateConn)

	return r
}

// Middleware validates the jwt token in c.Cookie(AuthCookie) or c.GetHeader(AuthHeader).
// If the token is not valid or expired, it will redirect the client to authURL.
// If the token is valid, it will set the user's data into the gin context
// at user.ContextUserKey.
func (r *Router) Middleware(secret string, authURL string) gin.HandlerFunc {
	return func(c *gin.Context) {

		isService := c.GetHeader(ServiceHostHeader)

		if isService == "True" {
			r.ServiceMiddleware(c)
			c.Next()
			return
		}

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
		id, idOk := claims["id"].(string)
		firstName, firstNameOk := claims[FirstNameClaim].(string)
		lastName, lastNameOk := claims[LastNameClaim].(string)

		// If any of the claims are invalid then redirect to authentication
		if !idOk || !firstNameOk || !lastNameOk {
			r.redirectToAuthService(c, authURL, fmt.Sprintf("invalid token claims: %v", claims))
			return
		}

		// The current transaction of the apm, adding the user id to the context.
		currentTarnasction := apm.TransactionFromContext(c.Request.Context())
		currentTarnasction.Context.SetUserID(id)
		currentTarnasction.Context.SetCustom(UserNameLabel, firstName+" "+lastName)

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

		c.Set(user.ContextUserKey, user.User{
			ID:        id,
			FirstName: firstName,
			LastName:  lastName,
		})

		c.Next()
	}
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
				fmt.Sprintf("authorization header is not legal. value should start with 'Bearer': %v", authArr[0]))
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
	c.Redirect(http.StatusTemporaryRedirect, authURL)
	c.Abort()
}

// ServiceMiddleware is a middleware for services use.
// First it extract the token from the Auth header and validate it with spike service
// Then it add the scopes to the context, and then checks if there is a delegator
// It check another header for that, and if its true it validate the delegator in the
// delegation service, and then adds it as delegator to the context.
func (r *Router) ServiceMiddleware(c *gin.Context) {

	tokenString := r.ExtractTokenFromHeader(c)
	if tokenString == "" {
		return
	}

	validateSpikeTokenRequest := &spb.ValidateTokenResquest{
		Token:    tokenString,
		Audience: "kartoffel", // TODO: change to clientID
	}

	spikeResponse, err := r.spikeClient.ValidateToken(c, validateSpikeTokenRequest)
	if err != nil {
		r.logger.Errorf("failure in spike-service integration: %v", err)
		c.AbortWithError(500, errors.New("internal error while authenticating the token"))
	}

	if !spikeResponse.Valid {
		r.logger.Infof("invalid token used: %v. Error: %v", tokenString, err)
		c.AbortWithError(401, fmt.Errorf("invalid token %v", err))
	}

	scopes := spikeResponse.Scopes

	// store scopes in context
	c.Set("scopes", scopes)

	// Find if the action is made on behalf of a user
	// Note: Later the scope should include the delegator
	delegatorID := c.GetHeader(AuthUserHeader)

	// if there is a delegator, validate him, then add him to context
	if delegatorID != "" {
		getUserByIDRequest := &dpb.GetUserByIDRequest{
			Id: delegatorID,
		}
		delegatorObj, err := r.delegateClient.GetUserByID(c, getUserByIDRequest)
		if err != nil {
			r.logger.Errorf("failure in delegation-service integration: %v", err)
			c.AbortWithError(500, errors.New("internal error while authenticating the delegator"))
		}

		delegator := delegatorObj.GetUser()

		c.Set(user.DelegatorKey, user.User{
			ID:        delegator.Id,
			FirstName: delegator.FirstName,
			LastName:  delegator.LastName,
		})
	}

	c.Next()

}

// ExtractTokenFromHeader extracts the token from the request header, and aborts with error if there is one.
func (r *Router) ExtractTokenFromHeader(c *gin.Context) string {
	authArr := strings.Fields(c.GetHeader(AuthHeader))

	// No authorization header sent
	if len(authArr) == 0 {
		// TODO: Should log here?
		c.AbortWithError(401, errors.New("no authorization header sent"))
		return ""
	}

	// The header value missing the correct prefix
	if authArr[0] != AuthHeaderBearer {
		c.AbortWithError(401, fmt.Errorf("authorization header is not legal. value should start with 'Bearer': %v", authArr[0]))
		return ""
	}

	// The value of the header doesn't contain the token
	if len(authArr) < 2 {
		c.AbortWithError(401, fmt.Errorf("no token sent in header %v", authArr))
		return ""
	}

	return authArr[1]
}
