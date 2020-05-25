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

	// ServiceAuthTypeValue is the value of service for AuthTypeHeader key
	DropboxAuthTypeValue = "Dropbox"

	// ServiceAuthCodeTypeValue is the value of service using the authorization code flow for AuthTypeHeader key
	ServiceAuthCodeTypeValue = "Service AuthCode"

	// ContextAppKey is the context key used to get and set the client's appID in the context.
	ContextAppKey = "appID"

	// ContextAppKey is the context key used to get and set the client's scopes in the context.
	ContextScopesKey = "scopes"

	// ContextAuthType is the context key used to get and set the auth type of the client in the context.
	ContextAuthType = "authType"

	// UploadScope is the scope required for upload
	UploadScope = "upload"

	// GetFileScope is the scope required for getting a file's metadata
	GetFileScope = "get"

	// ShareScope is the scope required for file share and unshare
	ShareScope = "share"

	// DownloadScope is the scope required for file upload
	DownloadScope = "download"

	// DriveAppID is the app ID of the drive client.
	DriveAppID = "drive"

	// DropboxAppID is the app ID of the dropbox services.
	DropboxAppID = "dropbox"

	// ClientCredentialsAuthType is the authentication type of client_credentials
	ClientCredentialsAuthType = "client_credentials"

	// ClientCredentialsAuthType is the authentication type of authorization_code
	AuthorizationCodeAuthType = "authorization_code"
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

		switch authType := c.GetHeader(AuthTypeHeader); authType {
		case ServiceAuthTypeValue:
			m.ClientCredentialsMiddleWare(c, requiredScope)
		case DropboxAuthTypeValue:
			m.ClientCredentialsMiddleWare(c, requiredScope)
		case ServiceAuthCodeTypeValue:
			m.AuthCodeMiddleWare(c, requiredScope)
		default:
			c.Next()
		}
		return
	}
}

// ClientCredentialsMiddleWare validate the token, and
// If scopes are nil, the client is the drive client which is fine. Else, the required
// scope should be included in the scopes array. If the required scope exists,and a
// delegator exists too, the function will set the context user to be the delegator.
func (m *Middleware) ClientCredentialsMiddleWare(c *gin.Context, requiredScope string) {
	c.Set(ContextAuthType, DropboxAuthTypeValue)

	scopes := m.extractScopes(c)
	if scopes == nil {
		loggermiddleware.LogError(
			m.logger,
			c.AbortWithError(
				http.StatusForbidden,
				fmt.Errorf("the givven is token not valid"),
			),
		)
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
			http.StatusForbidden,
			fmt.Errorf("the service is not allowed to do this operation - required Scope not supplied - client credentials"),
		),
	)
}

func (m *Middleware) AuthCodeMiddleWare(c *gin.Context, requiredScope string) {
	c.Set(ContextAuthType, ServiceAuthCodeTypeValue)
	spikeToken := m.extractAuthCodeToken(c)

	if spikeToken == nil {
		loggermiddleware.LogError(
			m.logger,
			c.AbortWithError(
				http.StatusForbidden,
				fmt.Errorf("the givven is token not valid"),
			),
		)
		return
	}

	scopes := spikeToken.GetScopes()
	user := spikeToken.GetUser()
	appID := spikeToken.GetAlias()

	// Checks the scopes, and if correct, register the user and the client ID.
	for _, scope := range scopes {
		if scope == requiredScope {
			m.register(c, user, appID)
			c.Next()
			return
		}
	}

	loggermiddleware.LogError(
		m.logger,
		c.AbortWithError(
			http.StatusForbidden,
			fmt.Errorf("the service is not allowed to do this operation - required Scope not supplied - authorization code"),
		),
	)
}

// extractAuthCodeToken extracts the auth-code token from the Auth header and validates
// it with spike service. Returns the extracted token.
func (m *Middleware) extractAuthCodeToken(c *gin.Context) *spb.ValidateAuthCodeTokenResponse {

	tokenString := m.extractTokenFromHeader(c)
	if tokenString == "" {
		c.Next()
		return nil
	}
	validateAuthCodeTokenRequest := &spb.ValidateAuthCodeTokenRequest{
		Token: tokenString,
	}

	spikeResponse, err := m.spikeClient.ValidateAuthCodeToken(c, validateAuthCodeTokenRequest)
	if err != nil {
		loggermiddleware.LogError(m.logger, c.AbortWithError(http.StatusInternalServerError,
			fmt.Errorf("internal error while authenticating the auth-code token: %v", err)))
		c.Next()
		return nil
	}

	if !spikeResponse.Valid {
		message := spikeResponse.GetMessage()
		loggermiddleware.LogError(m.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("invalid token %s", message)))

		c.Next()
		return nil
	}

	return spikeResponse
}

// extractScopes extracts the token from the Auth header and validates
// them with spike service. Returns the scopes.
func (m *Middleware) extractScopes(c *gin.Context) []string {

	tokenString := m.extractTokenFromHeader(c)
	if tokenString == "" {
		return nil
	}

	validateSpikeTokenRequest := &spb.ValidateTokenRequest{
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

// register saves the user and client ID into the context
func (m *Middleware) register(c *gin.Context, delegator *spb.User, clientID string) {

	c.Set(user.ContextUserKey, user.User{
		ID:        delegator.Id,
		FirstName: delegator.FirstName,
		LastName:  delegator.LastName,
		Source:    user.ExternalUserSource,
	})

	c.Set(ContextAppKey, clientID)
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

// validateRequiredScope checks if there is a specific scope in the context (unless it is the drive client).
func (m *Middleware) validateRequiredScope(c *gin.Context, requiredScope string) bool {

	appID := c.Value(ContextAppKey)
	if appID == DriveAppID {
		return true
	}

	// temporerly
	authType := c.Value(ContextAuthType)
	if authType == DropboxAuthTypeValue {
		return true
	}
	// ----------

	contextScopes := c.Value(ContextScopesKey)
	var scopes []string

	switch v := contextScopes.(type) {
	case []string:
		scopes = v
	default:
		return false
	}

	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}

	return false
}
