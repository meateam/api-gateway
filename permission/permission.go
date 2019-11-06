package permission

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	"github.com/meateam/api-gateway/internal/util"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/user-service/proto"
	pool "github.com/processout/grpc-go-pool"
	"github.com/sirupsen/logrus"
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
	permissionConnPool *pool.Pool
	fileConnPool       *pool.Pool
	userConnPool       *pool.Pool
	logger             *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	permissionConnPool *pool.Pool,
	fileConnPool *pool.Pool,
	userConnPool *pool.Pool,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.permissionConnPool = permissionConnPool
	r.fileConnPool = fileConnPool
	r.userConnPool = userConnPool
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

	permissions, err := GetFilePermissions(c.Request.Context(), fileID, r.permissionConnPool, r.fileConnPool)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, permissions)
}

// CreateFilePermission creates a permission for a given file
// File id is extracted from url params, role is extracted from request body.
//nolint:gocyclo
func (r *Router) CreateFilePermission(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	permission := &createPermissionRequest{}
	if c.ShouldBindJSON(permission) != nil {
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

	userClient, userClientConn := r.GetUserClient(c)
	if userClient == nil || userClientConn == nil {
		return
	}
	defer userClientConn.Close()

	userExists, err := userClient.GetUserByID(c.Request.Context(), &upb.GetByIDRequest{Id: permission.UserID})
	if err != nil {
		userClientConn.Unhealthy()

		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if userExists.GetUser() == nil || userExists.GetUser().GetId() != permission.UserID {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !r.HandleUserFilePermission(c, fileID, CreateFilePermissionRole) {
		return
	}

	createdPermission, err := CreatePermission(c.Request.Context(), r.permissionConnPool, Permission{
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

	permissionClient, permissionClientConn := r.GetPermissionClient(c)
	if permissionClient == nil || permissionClientConn == nil {
		return
	}
	defer permissionClientConn.Close()

	if userID == reqUser.ID {
		permission, err := permissionClient.GetPermission(c.Request.Context(),
			&ppb.GetPermissionRequest{FileID: fileID, UserID: reqUser.ID})
		if err != nil {
			permissionClientConn.Unhealthy()

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
	permission, err := permissionClient.DeletePermission(c.Request.Context(), deleteRequest)
	if err != nil {
		permissionClientConn.Unhealthy()

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
		r.fileConnPool,
		r.permissionConnPool,
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
	permissionConnPool *pool.Pool,
	fileID string,
	userID string,
	role ppb.Role) (*ppb.IsPermittedResponse, error) {
	permissionClient, permissionClientConn, err := util.GetPermissionClient(ctx, permissionConnPool)
	if err != nil {
		return nil, err
	}
	defer permissionClientConn.Close()

	isPermittedRequest := &ppb.IsPermittedRequest{
		FileID: fileID,
		UserID: userID,
		Role:   role,
	}
	isPermittedResponse, err := permissionClient.IsPermitted(ctx, isPermittedRequest)
	if err != nil {
		permissionClientConn.Unhealthy()

		return nil, err
	}

	return isPermittedResponse, nil
}

// CreatePermission creates permission in the permission-service.
func CreatePermission(ctx context.Context,
	permissionConnPool *pool.Pool,
	permission Permission) (*ppb.PermissionObject, error) {
	permissionClient, permissionClientConn, err := util.GetPermissionClient(ctx, permissionConnPool)
	if err != nil {
		return nil, err
	}
	defer permissionClientConn.Close()

	permissionRequest := &ppb.CreatePermissionRequest{
		FileID: permission.FileID,
		UserID: permission.UserID,
		Role:   ppb.Role(ppb.Role_value[permission.Role]),
	}

	createdPermission, err := permissionClient.CreatePermission(ctx, permissionRequest)
	if err != nil {
		permissionClientConn.Unhealthy()

		return nil, err
	}

	return createdPermission, nil
}

// GetFilePermissions returns all derived user permissions of a file.
func GetFilePermissions(ctx context.Context,
	fileID string,
	permissionConnPool *pool.Pool,
	fileConnPool *pool.Pool) ([]*ppb.GetFilePermissionsResponse_UserRole, error) {
	permissionsMap := make(map[string]*ppb.GetFilePermissionsResponse_UserRole, 1)
	permissions := make([]*ppb.GetFilePermissionsResponse_UserRole, 0, 1)
	currentFileID := fileID

	permissionClient, permissionClientConn, err := util.GetPermissionClient(ctx, permissionConnPool)
	if err != nil {
		return nil, err
	}
	defer permissionClientConn.Close()

	fileClient, fileClientConn, err := util.GetFileClient(ctx, fileConnPool)
	if err != nil {
		return nil, err
	}
	defer fileClientConn.Close()

	for {
		permissionsRequest := &ppb.GetFilePermissionsRequest{FileID: currentFileID}
		permissionsResponse, err := permissionClient.GetFilePermissions(ctx, permissionsRequest)
		if err != nil && status.Code(err) != codes.Unimplemented {
			permissionClientConn.Unhealthy()

			return nil, err
		}

		currentFile, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: currentFileID})
		if err != nil {
			fileClientConn.Unhealthy()

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

// GetPermissionClient returns a permission service client and its connection from the pool and handles errors.
func (r *Router) GetPermissionClient(c *gin.Context) (ppb.PermissionClient, *pool.ClientConn) {
	client, clientConn, err := util.GetPermissionClient(c.Request.Context(), r.permissionConnPool)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusServiceUnavailable, err))

		return nil, nil
	}

	return client, clientConn
}

// GetFileClient returns a file service client and its connection from the pool and handles errors.
func (r *Router) GetFileClient(c *gin.Context) (fpb.FileServiceClient, *pool.ClientConn) {
	client, clientConn, err := util.GetFileClient(c.Request.Context(), r.fileConnPool)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusServiceUnavailable, err))

		return nil, nil
	}

	return client, clientConn
}

// GetUserClient returns a user service client and its connection from the pool and handles errors.
func (r *Router) GetUserClient(c *gin.Context) (upb.UsersClient, *pool.ClientConn) {
	client, clientConn, err := util.GetUserClient(c.Request.Context(), r.userConnPool)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusServiceUnavailable, err))

		return nil, nil
	}

	return client, clientConn
}
