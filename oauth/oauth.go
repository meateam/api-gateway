package oauth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/factory"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/api-gateway/utils"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	spb "github.com/meateam/spike-service/proto/spike-service"
	usrpb "github.com/meateam/user-service/proto/users"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.elastic.co/apm"
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

	// UpdatePermitStatusScope is the scope name required for updating a permit's scope
	UpdatePermitStatusScope = "status"

	// AuthTypeHeader is the key of the service-host header
	AuthTypeHeader = "Auth-Type"

	// DropboxAuthTypeValue is the value of the AuthTypeHeader key for the Dropbox services
	DropboxAuthTypeValue = "Dropbox"

	// CargoAuthTypeValue is the value of the AuthTypeHeader key for the Cargo services
	CargoAuthTypeValue = "Cargo"

	// FalconAuthTypeValue is the value of the AuthTypeHeader key for the Falcon services
	FalconAuthTypeValue = "Falcon"

	// ServiceAuthCodeTypeValue is the value of service using the authorization code flow for AuthTypeHeader key
	ServiceAuthCodeTypeValue = "Service AuthCode"

	// ContextAppKey is the context key used to get and set the client's appID in the context.
	ContextAppKey = "appID"

	// ContextScopesKey is the context key used to get and set the client's scopes in the context.
	ContextScopesKey = "scopes"

	// ContextAuthType is the context key used to get and set the auth type of the client in the context.
	ContextAuthType = "authType"

	// UploadScope is the scope required for upload
	UploadScope = "upload"

	// GetFileScope is the scope required for getting a file's metadata
	GetFileScope = "get_metadata"

	// ShareScope is the scope required for file share and unshare
	ShareScope = "share"

	// DownloadScope is the scope required for file download
	DownloadScope = "download"

	// DeleteScope is the scope required for file deletion
	DeleteScope = "delete"

	// DriveAppID is the app ID of the drive client.
	DriveAppID = "drive"

	// DropboxAppID is the app ID of the dropbox client.
	DropboxAppID = "dropbox"

	// CargoAppID is the app ID of the cargo client.
	CargoAppID = "cargo"

	// FalconAppID is the app ID of the falcon client.
	FalconAppID = "falcon"

	// QueryAppID is a constant for queryAppId parameter in a request.
	// If exists, the files returned will only belong to the app of QueryAppID.
	QueryAppID = "appId"

	// TransactionClientLabel is the label of the custom transaction field : client-name.
	TransactionClientLabel = "client"

	// ConfigTomcalDest is the name of the environment variable containing the tomcal dest name.
	ConfigTomcalDest = "tomcal_dest_value"

	// ConfigCtsDest is the name of the environment variable containing the cts dest name.
	ConfigCtsDest = "cts_dest_value"
)

const (
	NoAuthActionDownload string = "0"
	NoAuthActionUpload   string = "1"
)

var (
	AllowedNoAuthAppsAndActions = map[string][]string{FalconAppID: {NoAuthActionDownload}}
)

// Middleware is a structure that handles the authentication middleware.
type Middleware struct {
	// SpikeClientFactory
	spikeClient factory.SpikeClientFactory

	// UserClientFactory
	userClient factory.UserClientFactory

	logger *logrus.Logger
}

// NewOAuthMiddleware generates a middleware.
// If logger is non-nil then it will be set as-is,
// otherwise logger would default to logrus.New().
func NewOAuthMiddleware(
	spikeConn *grpcPoolTypes.ConnPool,
	userConn *grpcPoolTypes.ConnPool,
	logger *logrus.Logger,
) *Middleware {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	m := &Middleware{logger: logger}

	m.spikeClient = func() spb.SpikeClient {
		return spb.NewSpikeClient((*spikeConn).Conn())
	}

	m.userClient = func() usrpb.UsersClient {
		return usrpb.NewUsersClient((*userConn).Conn())
	}

	return m
}

// AuthorizationScopeMiddleware creates a middleware function that checks the scopes in context.
// If the request is not from a service (AuthTypeHeader), Next will be immediately called.
// If scopes are nil, the client is the drive client which is fine. Else, the required
// scope should be included in the scopes array. If the required scope exists,and a
// delegator exists too, the function will set the context user to be the delegator.
func (m *Middleware) AuthorizationScopeMiddleware(requiredScope string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		appId := ctx.Query(QueryAppID)

		if ctx.Query("alt") == "media" && IsAppAllowedNoAuthAction(appId, NoAuthActionDownload) {
			ctx.Next()
			return
		}

		authType := ctx.GetHeader(AuthTypeHeader)
		ctx.Set(ContextAuthType, authType)

		var err error

		switch authType {
		case DropboxAuthTypeValue:
			err = m.serviceAuthorization(ctx, requiredScope)
		case CargoAuthTypeValue:
			err = m.serviceAuthorization(ctx, requiredScope)
		case FalconAuthTypeValue:
			err = m.serviceAuthorization(ctx, requiredScope)
		case ServiceAuthCodeTypeValue:
			err = m.authCodeAuthorization(ctx, requiredScope)
		default:
			ctx.Next()
		}

		if err != nil {
			loggermiddleware.LogError(m.logger, err)
		}
	}
}

// serviceAuthorization validates the token generated by spike with the client-creadentials auth type.
// Later, it extracts the scopes array from the token and return weather the required scope is in the scope array.
// If a delegator exists too, the function will set the context user to be the delegator.
func (m *Middleware) serviceAuthorization(ctx *gin.Context, requiredScope string) error {
	spikeToken, err := m.extractClientCredentialsToken(ctx)

	if err != nil {
		return err
	}

	scopes := spikeToken.GetScopes()

	ctx.Set(ContextScopesKey, scopes)

	authType := ctx.Value(ContextAuthType)
	var appID string
	switch authType {
	case CargoAuthTypeValue:
		appID = CargoAppID
	case FalconAuthTypeValue:
		appID = FalconAppID
	default:
		appID = DropboxAppID
	}

	// Checks the scopes, and if correct, store the user in the context.
	for _, scope := range scopes {
		if scope == requiredScope {
			err = m.storeDelegator(ctx)
			if err != nil {
				return err
			}

			ctx.Set(ContextAppKey, appID)
			SetApmClient(ctx, appID)

			ctx.Next()
			return nil
		}
	}

	return ctx.AbortWithError(
		http.StatusForbidden,
		fmt.Errorf("required scope '%s' is not supplied - '%s' authorization", requiredScope, appID),
	)
}

// AuthCodeAuthorization validates the token generated by spike with the authorization-code auth type.
// Later, it extracts the scopes array from the token and checks weather the required scope is in the scope array.
// If it is, it register the user and client ID to the context
func (m *Middleware) authCodeAuthorization(ctx *gin.Context, requiredScope string) error {
	spikeToken, err := m.extractAuthCodeToken(ctx)

	if err != nil {
		return err
	}

	scopes := spikeToken.GetScopes()
	user := spikeToken.GetUser()
	appID := spikeToken.GetAlias()

	ctx.Set(ContextScopesKey, scopes)

	// Checks the scopes, and if correct, register the user and the client ID.
	for _, scope := range scopes {
		if scope == requiredScope {
			m.register(ctx, user)

			ctx.Set(ContextAppKey, appID)
			SetApmClient(ctx, appID)

			ctx.Next()
			return nil
		}
	}

	return ctx.AbortWithError(
		http.StatusForbidden,
		fmt.Errorf("required scope '%s' is not supplied - authorization code", requiredScope),
	)
}

// extractAuthCodeToken extracts the auth-code token from the Auth header and validates
// it with spike service. Returns the extracted token.
func (m *Middleware) extractAuthCodeToken(ctx *gin.Context) (*spb.ValidateAuthCodeTokenResponse, error) {

	token, err := ExtractTokenFromHeader(ctx)
	if err != nil {
		return nil, err
	}

	validateAuthCodeTokenRequest := &spb.ValidateAuthCodeTokenRequest{
		Token: token,
	}

	spikeResponse, err := m.spikeClient().ValidateAuthCodeToken(ctx, validateAuthCodeTokenRequest)
	if err != nil {
		return nil, ctx.AbortWithError(http.StatusInternalServerError,
			fmt.Errorf("internal error while authenticating the auth-code token: %v", err))
	}

	if !spikeResponse.Valid {
		message := spikeResponse.GetMessage()
		return nil, ctx.AbortWithError(http.StatusUnauthorized, fmt.Errorf("invalid token: %s", message))
	}

	return spikeResponse, nil
}

// extractClientCredentialsToken extracts the token from the Auth header and validates it with spike service.
// Returns the extracted token.
func (m *Middleware) extractClientCredentialsToken(ctx *gin.Context) (*spb.ValidateTokenResponse, error) {
	token, err := ExtractTokenFromHeader(ctx)
	if err != nil {
		return nil, err
	}

	validateSpikeTokenRequest := &spb.ValidateTokenRequest{
		Token: token,
	}

	spikeResponse, err := m.spikeClient().ValidateToken(ctx, validateSpikeTokenRequest)
	if err != nil {
		return nil, ctx.AbortWithError(http.StatusInternalServerError,
			fmt.Errorf("internal error while authenticating the client-credentias token: %v", err))
	}

	if !spikeResponse.Valid {
		message := spikeResponse.GetMessage()
		return nil, ctx.AbortWithError(http.StatusUnauthorized, fmt.Errorf("invalid token: %s", message))
	}

	return spikeResponse, nil
}

// storeDelegator checks if there is a delegator, and if so it validates the
// delegator with the user service.
// Then it sets the User in the request's context to be the delegator.
func (m *Middleware) storeDelegator(ctx *gin.Context) error {
	// Check if the action is made on behalf of a user
	delegatorID := ctx.GetHeader(AuthUserHeader)

	authType := ctx.Value(ContextAuthType)
	var destination string
	switch authType {
	case CargoAuthTypeValue:
		destination = viper.GetString(ConfigCtsDest)
	case DropboxAuthTypeValue:
		destination = viper.GetString(ConfigTomcalDest)
	default:
		destination = ""
	}

	// If there is a delegator, validate him, then add him to the context
	if delegatorID != "" {
		getUserByIDRequest := &usrpb.GetByIDRequest{
			Id:          delegatorID,
			Destination: destination,
		}
		delegatorObj, err := m.userClient().GetUserByID(ctx.Request.Context(), getUserByIDRequest)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ctx.AbortWithError(http.StatusUnauthorized,
					fmt.Errorf("delegator: %v is not found", delegatorID))
			}

			return ctx.AbortWithError(http.StatusUnauthorized,
				fmt.Errorf("internal error while authenticating the delegator: %v", err))
		}

		delegator := delegatorObj.GetUser()

		authenticatedUser := user.User{
			ID:          delegator.GetId(),
			FirstName:   delegator.GetFirstName(),
			LastName:    delegator.GetLastName(),
			Source:      user.ExternalUserSource,
			DisplayName: delegator.GetHierarchyFlat(),
		}

		user.SetApmUser(ctx, authenticatedUser)
		ctx.Set(user.ContextUserKey, authenticatedUser)
	}

	return nil
}

// register saves the user and client ID into the context
func (m *Middleware) register(ctx *gin.Context, delegator *spb.User) {

	authenticatedUser := user.User{
		ID:        delegator.GetId(),
		FirstName: delegator.GetFirstName(),
		LastName:  delegator.GetLastName(),
		Source:    user.InternalUserSource,
	}

	user.SetApmUser(ctx, authenticatedUser)
	ctx.Set(user.ContextUserKey, authenticatedUser)
}

// ExtractTokenFromHeader extracts the token from the header and returns it as a string
func ExtractTokenFromHeader(ctx *gin.Context) (string, error) {
	authArr := strings.Fields(ctx.GetHeader(AuthHeader))

	// No authorization header sent
	if len(authArr) == 0 {
		return "", ctx.AbortWithError(http.StatusUnauthorized, fmt.Errorf("no authorization header sent"))
	}

	// The header value missing the correct prefix
	if authArr[0] != AuthHeaderBearer {
		return "", ctx.AbortWithError(http.StatusUnauthorized, fmt.Errorf(
			"authorization header is invalid. Value should start with 'Bearer'"))
	}

	// The value of the header doesn't contain the token
	if len(authArr) < 2 {
		return "", ctx.AbortWithError(http.StatusUnauthorized, fmt.Errorf("no token sent in header %v", authArr))

	}

	return authArr[1], nil
}

// ValidateRequiredScope checks if there is a specific scope in the context (unless it is the drive client).
func (m *Middleware) ValidateRequiredScope(ctx *gin.Context, requiredScope string) bool {

	appID := ctx.Value(ContextAppKey)
	if appID == DriveAppID {
		return true
	}

	contextScopes := ctx.Value(ContextScopesKey)
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

// SetApmClient adds a clientID to the current apm transaction.
func SetApmClient(ctx *gin.Context, clientID string) {
	currentTransaction := apm.TransactionFromContext(ctx.Request.Context())
	currentTransaction.Context.SetCustom(TransactionClientLabel, clientID)
}

func IsAppAllowedNoAuthAction(appID string, action string) bool {
	return utils.StringInSlice(action, AllowedNoAuthAppsAndActions[appID])
}
