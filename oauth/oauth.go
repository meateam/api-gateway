package oauth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	dpb "github.com/meateam/delegation-service/proto/delegation-service"
	spb "github.com/meateam/spike-service/proto/spike-service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// AuthHeader is the key of the authorization header.
	AuthHeader = "Authorization"

	// AuthHeaderBearer is the prefix for the authorization token in AuthHeader.
	AuthHeaderBearer = "Bearer"

	// AuthUserHeader is the key of the header which indicates whether an action is made on behalf of a user
	AuthUserHeader = "Auth-User"

	// DelegatorKey is the context key used to get and set the delegator's data in the context.
	DelegatorKey = "Delegator"

	// OutAdminScope is the scope name required for uploading,
	// downloading, and sharing files for an out-source user
	OutAdminScope = "externalAdmin"

	// ScopesKey  is the context key used to get the service scopes in the context
	ScopesKey = "Scopes"

	// UpdatePermitStatusScope is the scope name required for updating a permit's scope
	UpdatePermitStatusScope = "status"

	// AuthTypeHeader is the key of the servive-host header
	AuthTypeHeader = "Auth-Type"

	// ServiceAuthTypeValue is the value of service for AuthTypeHeader key
	ServiceAuthTypeValue = "Service"
)

// Middleware is a structure that handels the authentication middleware.
type Middleware struct {
	spikeClient    spb.SpikeClient
	delegateClient dpb.DelegationClient
	logger         *logrus.Logger
}

// NewOAuthMiddleware creates a new Router. If logger is non-nil then it will be
// set as-is, otherwise logger would default to logrus.New().
func NewOAuthMiddleware(
	spikeConn *grpc.ClientConn,
	delegateConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Middleware {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Middleware{logger: logger}

	r.spikeClient = spb.NewSpikeClient(spikeConn)

	r.delegateClient = dpb.NewDelegationClient(delegateConn)

	return r
}

// ScopeMiddleware creates a middleware function that checks the scopes in context.
// If the request is not from a service (AuthTypeHeader), Next will be immediately called.
// If scopes are nil, the client is the drive client which is fine. Else, the required
// scope should be included in the scopes array. If the required scope exists,and a
// delegator exists too, the function will set the context user to be the delegator.
func (m *Middleware) ScopeMiddleware(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {

		isService := c.GetHeader(AuthTypeHeader)

		// If this is not a service, the user was already authenticated in Auth's UserMiddlewere
		if isService != ServiceAuthTypeValue {
			c.Next()
			return
		}

		scopes := m.extractScopes(c)
		if scopes == nil {
			c.Next()
			return
		}
		for _, scope := range scopes {
			if scope == requiredScope {
				m.storeDelegator(c)
				c.Next()
				return
			}
		}
		loggermiddleware.LogError(
			m.logger,
			c.AbortWithError(
				http.StatusUnauthorized,
				fmt.Errorf("the service is not allowed to do this opperation"),
			),
		)
	}
}

// ExtractScopes extract the token from the Auth header and validate
// it with spike service. Then it add the scopes to the context.
// and then checks if there is a delegator
// It check another header for that, and if its true it validate the delegator in the
// delegation service, and then adds it as delegator to the context.
func (m *Middleware) extractScopes(c *gin.Context) []string {

	tokenString := m.extractTokenFromHeader(c)
	if tokenString == "" {
		return nil
	}

	validateSpikeTokenRequest := &spb.ValidateTokenResquest{
		Token: tokenString,
	}

	spikeResponse, err := m.spikeClient.ValidateToken(c, validateSpikeTokenRequest)
	if err != nil {
		loggermiddleware.LogError(m.logger, c.AbortWithError(http.StatusInternalServerError,
			fmt.Errorf("internal error while authenticating the token: %v", err)))
		return nil
	}

	if !spikeResponse.Valid {
		message := spikeResponse.GetMessage()
		loggermiddleware.LogError(m.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("invalid token %s", message)))

		return nil
	}

	scopes := spikeResponse.GetScopes()
	return scopes
}

// storeDelegator checks if there is a delegator and if so it validate the
// delegator with the delegation service, then adds it as delegator to the context.
func (m *Middleware) storeDelegator(c *gin.Context) {
	// Find if the action is made on behalf of a user
	// Note: Later the scope should include the delegator
	delegatorID := c.GetHeader(AuthUserHeader)

	// if there is a delegator, validate him, then add him to context
	if delegatorID != "" {
		getUserByIDRequest := &dpb.GetUserByIDRequest{
			Id: delegatorID,
		}
		delegatorObj, err := m.delegateClient.GetUserByID(c.Request.Context(), getUserByIDRequest)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				loggermiddleware.LogError(m.logger,
					c.AbortWithError(http.StatusUnauthorized,
						fmt.Errorf("Delegator: %v is not found", delegatorID)))
				return
			}
			loggermiddleware.LogError(m.logger, c.AbortWithError(http.StatusInternalServerError,
				fmt.Errorf("internal error while authenticating the delegator: %v", err)))
			return
		}

		delegator := delegatorObj.GetUser()

		c.Set(user.ContextUserKey, user.User{
			ID:        delegator.Id,
			FirstName: delegator.FirstName,
			LastName:  delegator.LastName,
			Source:    user.ExternalUserSource,
		})
	}
}

func (m *Middleware) extractTokenFromHeader(c *gin.Context) string {
	authArr := strings.Fields(c.GetHeader(AuthHeader))

	// No authorization header sent
	if len(authArr) == 0 {
		loggermiddleware.LogError(m.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("no authorization header sent")))
		return ""
	}

	// The header value missing the correct prefix
	if authArr[0] != AuthHeaderBearer {
		loggermiddleware.LogError(m.logger, c.AbortWithError(http.StatusUnauthorized, fmt.Errorf(
			"authorization header is not legal. value should start with 'Bearer': %v", authArr[0])))
		return ""
	}

	// The value of the header doesn't contain the token
	if len(authArr) < 2 {
		loggermiddleware.LogError(m.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("no token sent in header %v", authArr)))
		return ""
	}

	return authArr[1]
}
