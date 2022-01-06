package dropbox

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/permission"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/api-gateway/utils"
	drp "github.com/meateam/dropbox-service/proto/dropbox"
	fpb "github.com/meateam/file-service/proto/file"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"

	// ParamUserID is the name of the user id param in URL.
	ParamUserID = "id"

	// ParamApproverID is the name of the approver id param in the URL.
	ParamApproverID = "approverID"

	// QueryGetAll is the name of the query if get for all users
	QueryGetAll = "all"

	// HeaderFileID is the context key used to get fileId.
	HeaderFileID = "fileID"

	// HeaderDestionation is the context key used to get and set the external destination.
	HeaderDestionation = "destination"

	// ConfigTomcalDest is the name of the environment variable containing the tomcal dest name.
	ConfigTomcalDest = "tomcal_dest_value"

	// ConfigCtsDest is the name of the environment variable containing the cts dest name.
	ConfigCtsDest = "cts_dest_value"

	// ParamPageNum is a constant for the requested page num in the pagination.
	ParamPageNum = "pageNum"

	// ParamPageSize is a constant for the requested page size in the pagination.
	ParamPageSize = "pageSize"
)

type createExternalShareRequest struct {
	FileName       string   `json:"fileName"`
	Users          []User   `json:"users,omitempty"`
	Classification string   `json:"classification"`
	Info           string   `json:"info,omitempty"`
	Approvers      []string `json:"approvers,omitempty"`
	Destination    string   `json:"destination"`
	OwnerId        string   `json:"ownerId"`
}

// User struct
type User struct {
	ID       string `json:"id,omitempty"`
	FullName string `json:"full_name,omitempty"`
}

// Router is a structure that handles permission requests.
type Router struct {
	// DropboxClientFactory
	dropboxClient factory.DropboxClientFactory

	// PermissionClientFactory
	permissionClient factory.PermissionClientFactory

	// FileClientFactory
	fileClient factory.FileClientFactory

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
		return drp.NewDropboxClient((*dropboxConn).Conn())
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
	rg.GET("/transfersInfo", r.GetTransfersInfo)
	rg.PUT(fmt.Sprintf("/files/:%s/transfer", ParamFileID), r.CreateExternalShareRequest)

	rg.GET(fmt.Sprintf("/users/:%s/canApproveToUser/:approverID", ParamUserID), r.CanApproveToUser)
	rg.GET(fmt.Sprintf("/users/:%s/approverInfo", ParamUserID), r.GetApproverInfo)
}

// GetTransfersInfo is a route function for retrieving transfersInfo of a file
// File id is extracted from url params
func (r *Router) GetTransfersInfo(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	isGetAll := c.Query(QueryGetAll)
	fileID := c.GetHeader(HeaderFileID)
	pageNum := utils.StringToInt64((c.Query(ParamPageNum)))
	pageSize := utils.StringToInt64(c.Query(ParamPageSize))

	isAllUsers, err := strconv.ParseBool(isGetAll)
	if isGetAll != "" && err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("please enter a valid value for %s query", QueryGetAll))
		return
	}
	if isAllUsers && fileID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("please enter a header %s, if all query is true", HeaderFileID))
		return
	}

	if fileID != "" {
		if permission, _ := utils.HandleUserFilePermission(r.fileClient(), r.permissionClient(), c, fileID, permission.GetFilePermissionsRole); permission == "" {
			return
		}
	}

	transferRequest := &drp.GetTransfersInfoRequest{FileID: fileID, SharerID: reqUser.ID, PageNum: pageNum, PageSize: pageSize}
	if isAllUsers {
		transferRequest = &drp.GetTransfersInfoRequest{FileID: fileID, PageNum: pageNum, PageSize: pageSize}
	}

	transfersResponse, err := r.dropboxClient().GetTransfersInfo(c.Request.Context(), transferRequest)
	if err != nil && status.Code(err) != codes.Unimplemented {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, transfersResponse)
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
		c.String(http.StatusBadRequest, "%s is a required field", ParamFileID)
		return
	}

	if createRequest.Destination != viper.GetString(ConfigCtsDest) && createRequest.Destination != viper.GetString(ConfigTomcalDest) {
		c.String(http.StatusBadRequest, fmt.Sprintf("destination %s doesnt supported", createRequest.Destination))
		return
	}

	permission, err := permission.IsPermitted(c, r.permissionClient(), fileID, reqUser.ID, permission.GetFilePermissionsRole)
	if err != nil || !permission.GetPermitted() {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var userIDs []*drp.ApprovalUser
	for i := 0; i < len(createRequest.Users); i++ {
		user := &drp.ApprovalUser{
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
		OwnerID:        createRequest.OwnerId,
	})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, createRequestRes)
}

// CanApproveToUser is the request handler for GET /users/:userId/canApproveToUser/:approverID
// Requires a destination header
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

	destination := c.GetHeader(HeaderDestionation)
	if destination == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s header is required", HeaderDestionation))
		return
	}
	if destination != viper.GetString(ConfigCtsDest) && destination != viper.GetString(ConfigTomcalDest) {
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

	destination := c.GetHeader(HeaderDestionation)
	if destination == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s header is required", HeaderDestionation))
		return
	}
	if destination != viper.GetString(ConfigCtsDest) && destination != viper.GetString(ConfigTomcalDest) {
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
