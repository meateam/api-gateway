package permission

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID is the name of the file id param in URL
	ParamFileID = "file-id"

	// ParamPermissionID is the name of the permission id param in URL
	ParamPermissionID = "permission-id"

	// FormKeyRole is the key of role in post data
	FormKeyRole = "role"
)

// Router is a structure that handles permission requests.
type Router struct {
	permissionClient ppb.PermissionClient
	logger           *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	permissionConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.permissionClient = ppb.NewPermissionClient(permissionConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.POST("/files/:file-id/permissions", r.CreateFilePermission)
	rg.PUT("/files/:file-id/permissions/:permission-id", r.UpdateFilePermission)
	rg.GET("/files/:file-id/permissions", r.GetFilePermissions)
	rg.DELETE("/files/:file-id/permissions/:permission-id", r.DeleteFilePermission)
}

// GetFilePermissions is a route function for retrieving permissions of a file
// File id is extracted from url params
func (r *Router) GetFilePermissions(c *gin.Context) {
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

	isPermittedRequest := ppb.IsPermittedRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role_OWNER}
	isPermittedResponse, err := r.permissionClient.IsPermitted(c, &isPermittedRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if !isPermittedResponse.Permitted {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	permissionsRequest := ppb.GetFilePermissionsRequest{FileID: fileID}
	permissionsResponse, err := r.permissionClient.GetFilePermissions(c, &permissionsRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, permissionsResponse.Permissions)
}

// CreateFilePermission creates a permission for a given file
// File id is extracted from url params, role is extracted from form data
func (r *Router) CreateFilePermission(c *gin.Context) {
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

	isPermittedRequest := ppb.IsPermittedRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role_OWNER}
	isPermittedResponse, err := r.permissionClient.IsPermitted(c, &isPermittedRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if !isPermittedResponse.Permitted {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	value, exists := c.GetPostForm(FormKeyRole)
	if !exists {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	role, err := strconv.Atoi(value)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	permissionRequest := ppb.CreatePermissionRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role(role)}
	permission, err := r.permissionClient.CreatePermission(c, &permissionRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.String(http.StatusOK, permission.GetId())
}

// UpdateFilePermission updates a file permission
// File id is extracted from url params, role is extracted from form data
func (r *Router) UpdateFilePermission(c *gin.Context) {
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

	isPermittedRequest := ppb.IsPermittedRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role_WRITE}
	isPermittedResponse, err := r.permissionClient.IsPermitted(c, &isPermittedRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if !isPermittedResponse.Permitted {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	value, exists := c.GetPostForm(FormKeyRole)
	if !exists {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	role, err := strconv.Atoi(value)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	permissionRequest := ppb.CreatePermissionRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role(role)}
	permission, err := r.permissionClient.CreatePermission(c, &permissionRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.String(http.StatusOK, permission.GetId())
}

// DeleteFilePermission deletes a file permission
// File id and permission id are extracted from url params
func (r *Router) DeleteFilePermission(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param(ParamFileID)
	permissionID := c.Param(ParamPermissionID)
	if fileID == "" || permissionID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	isPermittedRequest := ppb.IsPermittedRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role_WRITE}
	isPermittedResponse, err := r.permissionClient.IsPermitted(c, &isPermittedRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if !isPermittedResponse.Permitted {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteRequest := ppb.DeletePermissionRequest{FileID: fileID, UserID: reqUser.ID}
	permission, err := r.permissionClient.DeletePermission(c, &deleteRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.String(http.StatusOK, permission.GetId())
}

// IsPermitted checks if the requesting user has a given role for the given file
// File id is extracted from url params
func (r *Router) IsPermitted(c *gin.Context) {
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

	isPermittedRequest := ppb.IsPermittedRequest{FileID: fileID, UserID: reqUser.ID, Role: ppb.Role_WRITE}
	isPermittedResponse, err := r.permissionClient.IsPermitted(c, &isPermittedRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if !isPermittedResponse.Permitted {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.String(http.StatusOK, strconv.FormatBool(isPermittedResponse.Permitted))
}
