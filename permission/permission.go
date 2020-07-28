package permission

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/user-service/proto/users"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"

	// QueryDeleteUserPermission is the id of the user to delete its permission to a file.
	QueryDeleteUserPermission = "userId"

	// GetFilePermissionsRole is the role that is required of the authenticated requester to have to be
	// permitted to make the GetFilePermissions action.
	GetFilePermissionsRole = ppb.Role_READ

	// CreateFilePermissionRole is the role that is required of the authenticated requester to have to be
	// permitted to make the CreateFilePermission action.
	CreateFilePermissionRole = ppb.Role_WRITE

	// DeleteFilePermissionRole is the role that is required of the authenticated requester to have to be
	// permitted to make the DeleteFilePermission action.
	DeleteFilePermissionRole = ppb.Role_WRITE
)

type createPermissionRequest struct {
	UserID   string `json:"userID,omitempty"`
	Role     string `json:"role,omitempty"`
	Override bool   `json:"override"`
}

// Permission is a struct that describes a user's permission to a file.
type Permission struct {
	UserID  string `json:"userID,omitempty"`
	FileID  string `json:"fileID,omitempty"`
	Role    string `json:"role,omitempty"`
	Creator string `json:"creator,omitempty"`
}

// Router is a structure that handles permission requests.
type Router struct {
	permissionClient ppb.PermissionClient
	fileClient       fpb.FileServiceClient
	userClient       upb.UsersClient
	oAuthMiddleware  *oauth.Middleware
	logger           *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	permissionConn *grpc.ClientConn,
	fileConn *grpc.ClientConn,
	userConnection *grpc.ClientConn,
	oAuthMiddleware *oauth.Middleware,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.permissionClient = ppb.NewPermissionClient(permissionConn)
	r.fileClient = fpb.NewFileServiceClient(fileConn)
	r.userClient = upb.NewUsersClient(userConnection)

	r.oAuthMiddleware = oAuthMiddleware

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	checkExternalAdminScope := r.oAuthMiddleware.ScopeMiddleware(oauth.OutAdminScope)

	rg.GET(fmt.Sprintf("/files/:%s/permissions", ParamFileID), r.GetFilePermissions)
	rg.PUT(fmt.Sprintf("/files/:%s/permissions", ParamFileID), checkExternalAdminScope, r.CreateFilePermission)
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

	if role, _ := r.HandleUserFilePermission(c, fileID, GetFilePermissionsRole); role == "" {
		return
	}

	permissions, err := GetFilePermissions(c.Request.Context(), fileID, r.permissionClient, r.fileClient)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Get File's metadata for its owner.
	file, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	filteredOwnerPermissions := make([]Permission, 0, len(permissions))
	for i := 0; i < len(permissions); i++ {
		if permissions[i].UserID != file.GetOwnerID() {
			filteredOwnerPermissions = append(filteredOwnerPermissions, permissions[i])
		}
	}

	permissions = filteredOwnerPermissions

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
		loggermiddleware.LogError(r.logger,
			c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("request has wrong format")))
		return
	}

	// Forbid a user to give himself any permission.
	if permission.UserID == reqUser.ID {
		loggermiddleware.LogError(r.logger,
			c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("a user cannot give himself permissions")))
		return
	}

	// Forbid creating a permission of NONE.
	switch ppb.Role(ppb.Role_value[permission.Role]) {
	case ppb.Role_NONE:
		loggermiddleware.LogError(r.logger,
			c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("permission type %s is not valid! ", permission.Role)))
		return
	default:
		break
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	file, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	// Forbid changing the file owner's permission.
	if file.GetOwnerID() == permission.UserID {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if role, _ := r.HandleUserFilePermission(c, fileID, CreateFilePermissionRole); role == "" {
		return
	}

	userExists, err := r.userClient.GetUserByID(c.Request.Context(), &upb.GetByIDRequest{Id: permission.UserID})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if userExists.GetUser() == nil || userExists.GetUser().GetId() != permission.UserID {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	createdPermission, err := CreatePermission(c.Request.Context(), r.permissionClient, Permission{
		FileID:  fileID,
		UserID:  permission.UserID,
		Role:    permission.Role,
		Creator: reqUser.ID,
	}, permission.Override)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, Permission{
		UserID:  createdPermission.GetUserID(),
		FileID:  createdPermission.GetFileID(),
		Role:    createdPermission.GetRole().String(),
		Creator: createdPermission.GetCreator(),
	})
}

// DeleteFilePermission deletes a file permission,
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

	// Get the user id to delete its permission the file, if no param
	// is given, default to delete the permission of the authenticated requester.
	userID, exists := c.GetQuery(QueryDeleteUserPermission)
	if !exists {
		userID = reqUser.ID
	}

	file, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	if userID == file.GetOwnerID() {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Check permission to delete the permission. Need to check only if the authenticated requester
	// requested to delete another user's permission, since he can do this operation with any permission
	// to himself.
	if userID != reqUser.ID {
		if role, _ := r.HandleUserFilePermission(c, fileID, DeleteFilePermissionRole); role == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	deleteRequest := &ppb.DeletePermissionRequest{FileID: fileID, UserID: userID}
	permission, err := r.permissionClient.DeletePermission(c.Request.Context(), deleteRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, Permission{
		UserID:  permission.GetUserID(),
		FileID:  permission.GetFileID(),
		Role:    permission.GetRole().String(),
		Creator: permission.GetCreator(),
	})
}

// HandleUserFilePermission checks if the requesting user has a given role for the given file
// File id is extracted from url params
func (r *Router) HandleUserFilePermission(
	c *gin.Context,
	fileID string,
	role ppb.Role) (string, *ppb.PermissionObject) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return "", nil
	}

	userFilePermission, foundPermission, err := file.CheckUserFilePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		fileID,
		role)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return "", nil
	}

	if userFilePermission == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return userFilePermission, foundPermission
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
	permission Permission, override bool) (*ppb.PermissionObject, error) {
	permissionRequest := &ppb.CreatePermissionRequest{
		FileID:   permission.FileID,
		UserID:   permission.UserID,
		Role:     ppb.Role(ppb.Role_value[permission.Role]),
		Creator:  permission.Creator,
		Override: override,
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
	fileClient fpb.FileServiceClient) ([]Permission, error) {
	permissionsMap := make(map[string]Permission, 1)
	permissions := make([]Permission, 0, 1)
	currentFileID := fileID

	for {
		permissionsRequest := &ppb.GetFilePermissionsRequest{FileID: currentFileID}
		permissionsResponse, err := permissionClient.GetFilePermissions(ctx, permissionsRequest)
		if err != nil && status.Code(err) != codes.NotFound {
			return nil, err
		}

		currentFile, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: currentFileID})
		if err != nil {
			return nil, err
		}

		for _, permission := range permissionsResponse.GetPermissions() {
			if _, ok := permissionsMap[permission.GetUserID()]; !ok {
				userRole := Permission{
					UserID:  permission.GetUserID(),
					Role:    permission.GetRole().String(),
					FileID:  currentFileID,
					Creator: permission.GetCreator(),
				}
				permissionsMap[permission.GetUserID()] = userRole
				permissions = append(permissions, userRole)
			}
		}

		if currentFile.GetParent() == "" {
			break
		}

		currentFileID = currentFile.GetParent()
	}

	return permissions, nil
}
