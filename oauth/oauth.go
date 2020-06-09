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

	// OutAdminScope is the scope name required for uploading,
	// downloading, and sharing files for an out-source user
	OutAdminScope = "externalAdmin"

	// UpdatePermitStatusScope is the scope name required for updating a permit's scope
	UpdatePermitStatusScope = "status"

	// AuthTypeHeader is the key of the service-host header
	AuthTypeHeader = "Auth-Type"

	// ServiceAuthTypeValue is the value of service for AuthTypeHeader key
	ServiceAuthTypeValue = "Service"
)

// Middleware is a structure that handles the authentication middleware.
type Middleware struct {
	spikeClient    spb.SpikeClient
	delegateClient dpb.DelegationClient
	logger         *logrus.Logger
}

// NewOAuthMiddleware generates a middleware.
// If logger is non-nil then it will be set as-is,
// otherwise logger would default to logrus.New().
func NewOAuthMiddleware(
	spikeConn *grpc.ClientConn,
	delegateConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Middleware {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	m := &Middleware{logger: logger}

	m.spikeClient = spb.NewSpikeClient(spikeConn)

	m.delegateClient = dpb.NewDelegationClient(delegateConn)

	return m
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
				fmt.Errorf("the service is not allowed to do this operation"),
			),
		)
	}
}

// extractScopes extracts the token from the Auth header and validates
// them with spike service. Returns the scopes.
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

	return spikeResponse.GetScopes()
}

// storeDelegator checks if there is a delegator, and if so it validates the
// delegator with the delegation service.
// Then it sets the User in the request's context to be the delegator.
func (m *Middleware) storeDelegator(c *gin.Context) {
	// Check if the action is made on behalf of a user
	delegatorID := c.GetHeader(AuthUserHeader)

	// If there is a delegator, validate him, then add him to the context
	if delegatorID != "" {
		getUserByIDRequest := &dpb.GetUserByIDRequest{
			Id: delegatorID,
		}
		delegatorObj, err := m.delegateClient.GetUserByID(c.Request.Context(), getUserByIDRequest)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				loggermiddleware.LogError(m.logger,
					c.AbortWithError(http.StatusUnauthorized,
						fmt.Errorf("delegator: %v is not found", delegatorID)))
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
			"authorization header is invalid. Value should start with 'Bearer'")))
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
