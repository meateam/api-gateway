package dropbox

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/permission"
	"github.com/meateam/api-gateway/user"
	drp "github.com/meateam/dropbox-service/proto/dropbox"
	fpb "github.com/meateam/file-service/proto/file"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"

	// ParamReqID is the name of the request id param in URL.
	ParamReqID = "requestId"

	// ParamUserID is the name of the user id param in URL.
	ParamUserID = "userId"

	// ParamApproverID is the name of the approver id param in the URL.
	ParamApproverID = "approverID"

	// ContextDestionationKey is the context key used to get and set the external destination.
	ContextDestionationKey = "destinationID"

	// TODO: change to appID
	// TomcalID is the destination of the dropbox.
	TomcalID = "z"

	// CtsID is the destination of the dropbox.
	CtsID = "c"
)

type createExternalShareRequest struct {
	FileName       string   `json:"fileName"`
	Users          []User   `json:"users,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Info           string   `json:"info,omitempty"`
	Approvers      []string `json:"approvers,omitempty"`
	Destination    string   `json:"destination"`
}

// User ...
type User struct {
	ID       string `json:"id,omitempty"`
	FullName string `json:"full_name,omitempty"`
}

type updatePermitStatusRequest struct {
	Status string `json:"status,omitempty"`
}

// Router is a structure that handles permission requests.
type Router struct {
	// DropboxClientFactory
	dropboxClient factory.DropboxClientFactory

	// FileClientFactory
	fileClient factory.FileClientFactory

	// PermissionClientFactory
	permissionClient factory.PermissionClientFactory

	oAuthMiddleware *oauth.Middleware
	logger          *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	dropboxConn *grpcPoolTypes.ConnPool,
	permissionConn *grpcPoolTypes.ConnPool,
	fileConn *grpcPoolTypes.ConnPool,
	oAuthMiddleware *oauth.Middleware,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.dropboxClient = func() drp.DropboxClient {
		return drp.NewDropboxClient((*permissionConn).Conn())
	}

	r.permissionClient = func() ppb.PermissionClient {
		return ppb.NewPermissionClient((*permissionConn).Conn())
	}

	r.fileClient = func() fpb.FileServiceClient {
		return fpb.NewFileServiceClient((*fileConn).Conn())
	}

	r.oAuthMiddleware = oAuthMiddleware

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	// checkStatusScope := r.oAuthMiddleware.AuthorizationScopeMiddleware(oauth.UpdatePermitStatusScope)

	rg.GET(fmt.Sprintf("/files/:%s/permits", ParamFileID), r.GetFilePermits)
	rg.PUT(fmt.Sprintf("/files/:%s/permits", ParamFileID), r.CreateExternalShareRequest)

	rg.GET(fmt.Sprintf("/users/:%s/canApproveToUser/:approverID", ParamUserID), r.CanApproveToUser)
	rg.GET(fmt.Sprintf("/users/:%s/approverInfo", ParamUserID), r.GetApproverInfo)
}

// GetFilePermits is a route function for retrieving permits (shared destination users) of a file
// File id is extracted from url params
func (r *Router) GetFilePermits(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !r.GetUserFilePermission(c, fileID, permission.GetFilePermissionsRole) {
		return
	}

	transferRequest := &drp.GetTransfersInfoRequest{FileID: fileID, UserID: reqUser.ID}
	transfersResponse, err := r.dropboxClient().GetTransfersInfo(c.Request.Context(), transferRequest)
	if err != nil && status.Code(err) != codes.Unimplemented {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	permits := transfersResponse.GetTo()

	c.JSON(http.StatusOK, permits)
}

// CreateExternalShareRequest creates permits for a given file and users
// File id is extracted from url params, role is extracted from request body.
func (r *Router) CreateExternalShareRequest(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	createRequest := &createExternalShareRequest{}
	if err := c.ShouldBindJSON(createRequest); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	destination := c.GetHeader(ContextDestionationKey)
	if destination == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s header is required", ContextDestionationKey))
		return
	}
	if destination != CtsID && destination != TomcalID {
		c.String(http.StatusBadRequest, fmt.Sprintf("destination %s doesnt supported", destination))
		return
	}

	if !r.GetUserFilePermission(c, fileID, permission.CreateFilePermissionRole) {
		return
	}

	var userIDs []*drp.User
	for i := 0; i < len(createRequest.Users); i++ {
		user := &drp.User{
			Id:   createRequest.Users[i].ID,
			Name: createRequest.Users[i].FullName,
		}
		userIDs = append(userIDs, user)
	}

	createRequestRes, err := r.dropboxClient().CreateRequest(c.Request.Context(), &drp.CreateRequestRequest{
		FileID:         fileID,
		FileName:       createRequest.FileName,
		SharerID:       reqUser.ID,
		Users:          userIDs,
		Classification: createRequest.Classification,
		Info:           createRequest.Info,
		Approvers:      createRequest.Approvers,
		Destination:    createRequest.Destination,
	})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, createRequestRes)
}

// GetUserFilePermission gets a gin context and the id of the requested file,
// returns true if the user is permitted to operate on the file.
// Returns false if the user isn't permitted to operate on it,
// Returns false if any error occurred and logs the error.
func (r *Router) GetUserFilePermission(c *gin.Context, fileID string, role ppb.Role) bool {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return false
	}

	userFilePermission, _, err := file.CheckUserFilePermission(c.Request.Context(),
		r.fileClient(),
		r.permissionClient(),
		reqUser.ID,
		fileID,
		role)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return false
	}

	if userFilePermission == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return userFilePermission != ""
}

// CanApproveToUser is the request handler for GET /users/:userId/canApproveToUser/:approverID
// must required a destination header
func (r *Router) CanApproveToUser(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	userID := c.Param(ParamUserID)
	if userID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s field is required", ParamUserID))
		return
	}

	approverID := c.Param(ParamApproverID)
	if approverID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s field is required", ParamApproverID))
		return
	}

	destination := c.GetHeader(ContextDestionationKey)
	if destination == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s header is required", ContextDestionationKey))
		return
	}
	if destination != CtsID && destination != TomcalID {
		c.String(http.StatusBadRequest, fmt.Sprintf("destination %s doesnt supported", destination))
		return
	}

	canApproveToUserRequest := &drp.CanApproveToUserRequest{
		ApproverID:  approverID,
		UserID:      userID,
		Destination: destination,
	}

	canApproveToUserInfo, err := r.dropboxClient().CanApproveToUser(c.Request.Context(), canApproveToUserRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, canApproveToUserInfo)
}

// GetApproverInfo is the request handler for GET /users/:id/approverInfo
func (r *Router) GetApproverInfo(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	userID := c.Param(ParamUserID)
	if userID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s field is required", ParamUserID))
		return
	}

	destination := c.GetHeader(ContextDestionationKey)
	if destination == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s header is required", ContextDestionationKey))
		return
	}
	if destination != CtsID && destination != TomcalID {
		c.String(http.StatusBadRequest, fmt.Sprintf("destination %s doesnt supported", destination))
		return
	}

	getApproverInfoRequest := &drp.GetApproverInfoRequest{
		Id:          userID,
		Destination: destination,
	}

	info, err := r.dropboxClient().GetApproverInfo(c.Request.Context(), getApproverInfoRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, info)
}
