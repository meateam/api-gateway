package permission

import (
	"fmt"
	"net/http"

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
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "fileId"

	// ParamPermissionID is the name of the permission id param in URL.
	ParamPermissionID = "permissionId"

	// QueryDeleteUserPermission is the id of the user to delete its permission to a file.
	QueryDeleteUserPermission = "userId"
)

type permission struct {
	UserID string `json:"userID,omitempty"`
	Role   string `json:"role,omitempty"`
}

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
	rg.GET(fmt.Sprintf("/files/:%s/permissions", ParamFileID), r.GetFilePermissions)
	rg.PUT(fmt.Sprintf("/files/:%s/permissions", ParamFileID), r.CreateFilePermission)
	rg.DELETE(fmt.Sprintf("/files/:%s/permissions/:%s", ParamFileID, ParamPermissionID), r.DeleteFilePermission)
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

	if permitted := r.IsPermitted(c, fileID, reqUser.ID, ppb.Role_READ); !permitted {
		return
	}

	permissionsRequest := &ppb.GetFilePermissionsRequest{FileID: fileID}
	permissionsResponse, err := r.permissionClient.GetFilePermissions(c, permissionsRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, permissionsResponse.Permissions)
}

// CreateFilePermission creates a permission for a given file
// File id is extracted from url params, role is extracted from request body
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

	if permitted := r.IsPermitted(c, fileID, reqUser.ID, ppb.Role_OWNER); !permitted {
		return
	}

	permission := &permission{}
	if err := c.ShouldBindJSON(permission); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	permissionRequest := &ppb.CreatePermissionRequest{
		FileID: fileID,
		UserID: permission.UserID,
		Role:   ppb.Role(ppb.Role_value[permission.Role]),
	}
	createdPermission, err := r.permissionClient.CreatePermission(c, permissionRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	createdPermission.Id = ""

	c.JSON(http.StatusOK, createdPermission)
}

// DeleteFilePermission deletes a file permission
// File id and permission id are extracted from url params
func (r *Router) DeleteFilePermission(c *gin.Context) {
	fileID := c.Param(ParamFileID)
	permissionID := c.Param(ParamPermissionID)
	if fileID == "" || permissionID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteUserPermission := reqUser.ID
	requiredRole := ppb.Role_OWNER
	userID, exists := c.GetQuery(QueryDeleteUserPermission)
	if exists {
		deleteUserPermission = userID
	}

	if reqUser.ID == deleteUserPermission {
		// Should be lowest permission that can be given to a user.
		requiredRole = ppb.Role_READ
	}
	if permitted := r.IsPermitted(c, fileID, deleteUserPermission, requiredRole); !permitted {
		return
	}
	deleteRequest := &ppb.DeletePermissionRequest{FileID: fileID, UserID: deleteUserPermission}
	permission, err := r.permissionClient.DeletePermission(c, deleteRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	permission.Id = ""
	c.JSON(http.StatusOK, permission)
}

// IsPermitted checks if the requesting user has a given role for the given file
// File id is extracted from url params
func (r *Router) IsPermitted(c *gin.Context, fileID string, userID string, role ppb.Role) bool {
	isPermittedRequest := &ppb.IsPermittedRequest{
		FileID: fileID,
		UserID: userID,
		Role:   role,
	}
	isPermittedResponse, err := r.permissionClient.IsPermitted(c, isPermittedRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return false
	}

	if !isPermittedResponse.Permitted {
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	return isPermittedResponse.GetPermitted()
}
