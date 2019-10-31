package permission

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"

	// ParamPermissionID is the name of the permission id param in URL.
	ParamPermissionID = "permissionId"

	// QueryDeleteUserPermission is the id of the user to delete its permission to a file.
	QueryDeleteUserPermission = "userId"

	// GetFilePermissionsRole is the role that is required of the authenticated requester to have to be
	// permitted to make the GetFilePermissions action.
	GetFilePermissionsRole = ppb.Role_READ

	// CreateFilePermissionRole is the role that is required of the authenticated requester to have to be
	// permitted to make the CreateFilePermission action.
	CreateFilePermissionRole = ppb.Role_OWNER

	// DeleteFilePermissionRole is the role that is required of the authenticated requester to have to be
	// permitted to make the DeleteFilePermission action.
	DeleteFilePermissionRole = ppb.Role_OWNER
)

type createPermissionRequest struct {
	UserID string `json:"userID,omitempty"`
	Role   string `json:"role,omitempty"`
}

// Permission is a struct that describes a user's permission to a file.
type Permission struct {
	UserID string
	FileID string
	Role   string
}

// Router is a structure that handles permission requests.
type Router struct {
	permissionClient ppb.PermissionClient
	fileClient       fpb.FileServiceClient
	logger           *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	permissionConn *grpc.ClientConn,
	fileConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.permissionClient = ppb.NewPermissionClient(permissionConn)
	r.fileClient = fpb.NewFileServiceClient(fileConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET(fmt.Sprintf("/files/:%s/permissions", ParamFileID), r.GetFilePermissions)
	rg.PUT(fmt.Sprintf("/files/:%s/permissions", ParamFileID), r.CreateFilePermission)
	rg.DELETE(fmt.Sprintf("/files/:%s/permissions", ParamFileID), r.DeleteFilePermission)
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

	if !r.HandleUserFilePermission(c, fileID, GetFilePermissionsRole) {
		return
	}

	permissions, err := GetFilePermissions(c.Request.Context(), fileID, r.permissionClient, r.fileClient)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, permissions)
}

// CreateFilePermission creates a permission for a given file
// File id is extracted from url params, role is extracted from request body.
func (r *Router) CreateFilePermission(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	permission := &createPermissionRequest{}
	if err := c.ShouldBindJSON(permission); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Forbid a user to give himself any permission.
	if permission.UserID == reqUser.ID {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Forbid creating a permission of NONE or OWNER or WRITE.
	switch ppb.Role(ppb.Role_value[permission.Role]) {
	case ppb.Role_NONE:
	case ppb.Role_OWNER:
	case ppb.Role_WRITE:
		c.AbortWithStatus(http.StatusBadRequest)
		return
	default:
		break
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !r.HandleUserFilePermission(c, fileID, CreateFilePermissionRole) {
		return
	}

	createdPermission, err := CreatePermission(c.Request.Context(), r.permissionClient, Permission{
		FileID: fileID,
		UserID: permission.UserID,
		Role:   permission.Role,
	})
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
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	userID, exists := c.GetQuery(QueryDeleteUserPermission)
	if !exists {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if userID == reqUser.ID {
		permission, err := r.permissionClient.GetPermission(c.Request.Context(),
			&ppb.GetPermissionRequest{FileID: fileID, UserID: reqUser.ID})
		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		if permission.GetRole() == ppb.Role_OWNER {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
	}

	if userID != reqUser.ID && !r.HandleUserFilePermission(c, fileID, DeleteFilePermissionRole) {
		return
	}

	deleteRequest := &ppb.DeletePermissionRequest{FileID: fileID, UserID: userID}
	permission, err := r.permissionClient.DeletePermission(c.Request.Context(), deleteRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	permission.Id = ""
	c.JSON(http.StatusOK, permission)
}

// HandleUserFilePermission checks if the requesting user has a given role for the given file
// File id is extracted from url params
func (r *Router) HandleUserFilePermission(c *gin.Context, fileID string, role ppb.Role) bool {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return false
	}

	isPermittedResponse, err := file.CheckUserFilePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		fileID,
		role)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return false
	}

	if !isPermittedResponse {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return isPermittedResponse
}

// IsPermitted checks if the userID has a permission with role for fileID.
func IsPermitted(ctx context.Context,
	permissionClient ppb.PermissionClient,
	fileID string,
	userID string,
	role ppb.Role) (*ppb.IsPermittedResponse, error) {
	isPermittedRequest := &ppb.IsPermittedRequest{
		FileID: fileID,
		UserID: userID,
		Role:   role,
	}
	isPermittedResponse, err := permissionClient.IsPermitted(ctx, isPermittedRequest)
	if err != nil {
		return nil, err
	}

	return isPermittedResponse, nil
}

// CreatePermission creates permission in the permission-service.
func CreatePermission(ctx context.Context,
	permissionClient ppb.PermissionClient,
	permission Permission) (*ppb.PermissionObject, error) {
	permissionRequest := &ppb.CreatePermissionRequest{
		FileID: permission.FileID,
		UserID: permission.UserID,
		Role:   ppb.Role(ppb.Role_value[permission.Role]),
	}
	createdPermission, err := permissionClient.CreatePermission(ctx, permissionRequest)
	if err != nil {
		return nil, err
	}

	return createdPermission, nil
}

// GetFilePermissions returns all derived user permissions of a file.
func GetFilePermissions(ctx context.Context,
	fileID string,
	permissionClient ppb.PermissionClient,
	fileClient fpb.FileServiceClient) ([]*ppb.GetFilePermissionsResponse_UserRole, error) {
	permissionsMap := make(map[string]*ppb.GetFilePermissionsResponse_UserRole, 1)
	permissions := make([]*ppb.GetFilePermissionsResponse_UserRole, 0, 1)
	currentFileID := fileID

	for {
		permissionsRequest := &ppb.GetFilePermissionsRequest{FileID: currentFileID}
		permissionsResponse, err := permissionClient.GetFilePermissions(ctx, permissionsRequest)
		if err != nil && status.Code(err) != codes.Unimplemented {
			return nil, err
		}

		currentFile, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: currentFileID})
		if err != nil {
			return nil, err
		}

		for _, permission := range permissionsResponse.GetPermissions() {
			if _, ok := permissionsMap[permission.GetUserID()]; !ok {
				permissionsMap[permission.GetUserID()] = permission
				permissions = append(permissions, permission)
			}
		}

		if currentFile.GetParent() == "" {
			break
		}

		currentFileID = currentFile.GetParent()
	}

	return permissions, nil
}
