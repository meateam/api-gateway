package fav

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	fvpb "github.com/meateam/fav-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/meateam/api-gateway/file"

)

const (
	// CreateFavFileByIDRole is the role that is required of the of the authenticated requester to have to be
	// permitted to make the CreateFavorite action.
	CreateFavFileByIDRole = ppb.Role_READ

	// DeleteFavFileByIDRole is the role that is required of the authenticated requester to have to be
	// permitted to make the DeleteFavFileByIDRole action.
	DeleteFavFileByIDRole = ppb.Role_READ
)

// Fav is a struct of favorite file
type Fav struct {
	UserID string `json:"userID,omitempty"`
	FileID string `json:"fileID,omitempty"`
}

// Router is a structure that handles favorite requests.
type Router struct {
	favClient       factory.FavClientFactory
	fileClient 		factory.FileClientFactory
	permissionClient factory.PermissionClientFactory
	oAuthMiddleware *oauth.Middleware
	logger          *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the fav Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	favConn *grpcPoolTypes.ConnPool,
	fileConn *grpcPoolTypes.ConnPool,
	permissionConn *grpcPoolTypes.ConnPool,
	oAuthMiddleware *oauth.Middleware,
	logger *logrus.Logger) *Router {

	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.favClient = func() fvpb.FavoriteClient {
		return fvpb.NewFavoriteClient((*favConn).Conn())
	}

	r.fileClient = func() fpb.FileServiceClient {
		return fpb.NewFileServiceClient((*fileConn).Conn())
	}

	r.permissionClient = func() ppb.PermissionClient {
		return ppb.NewPermissionClient((*permissionConn).Conn())
	}

	r.oAuthMiddleware = oAuthMiddleware

	return r
	
}

//Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.POST(fmt.Sprintf("/fav/:id"), r.CreateFav)
	rg.DELETE(fmt.Sprintf("/fav/:id"), r.DeleteFav)
}

// CreateFav creates a favorite for a given file.
// FileID is extracted from url params.
func (r *Router) CreateFav(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param(file.ParamFileID)
	if fileID == "" {
		c.String(http.StatusBadRequest, file.FileIDIsRequiredMessage)
		return
	}

	if role, _ := r.HandleUserFilePermission(c, fileID, CreateFavFileByIDRole); role == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// An app can create a favorite only if the request is from the Drive.
	ctxAppID := c.Value(oauth.ContextAppKey).(string)
	if (ctxAppID != oauth.DriveAppID) {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusForbidden, fmt.Errorf("failed creating favorite, request has to be only from the Drive")))
		return
	}

	createReq := &fvpb.CreateFavoriteRequest{FileID: fileID, UserID: reqUser.ID}
	createdResponse, err := r.favClient().CreateFavorite(c.Request.Context(), createReq)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, Fav{
		UserID: createdResponse.UserID,
		FileID: createdResponse.FileID,
	})

}

// DeleteFav deletes a favorite by the fileID extracted from query params.
// A user can only delete his own fav object.
func (r *Router) DeleteFav(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param(file.ParamFileID)
	if fileID == "" {
		c.String(http.StatusBadRequest, file.FileIDIsRequiredMessage)
		return
	}


	if role, _ := r.HandleUserFilePermission(c, fileID, DeleteFavFileByIDRole); role == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteRequest := &fvpb.DeleteFavoriteRequest{FileID: fileID, UserID: reqUser.ID}
	fav, err := r.favClient().DeleteFavorite(c.Request.Context(), deleteRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, Fav{
		UserID: fav.UserID,
		FileID: fav.FileID,
	})

}

// HandleUserFilePermission gets the id of the requested file, and the required role.
// Returns the user role as a string, and the permission if the user is permitted
// to operate on the file, and `"", nil` if not.
func (r *Router) HandleUserFilePermission(
	c *gin.Context,
	fileID string,
	role ppb.Role) (string, *ppb.PermissionObject) {
	reqUser := user.ExtractRequestUser(c)

	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return "", nil
	}

	userStringRole, foundPermission, err := CheckUserFilePermission(c.Request.Context(),
		r.fileClient(),
		r.permissionClient(),
		reqUser.ID,
		fileID,
		role)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return "", nil
	}

	if userStringRole == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return userStringRole, foundPermission
}

// CheckUserFilePermission checks if userID is permitted to fileID with the wanted role.
// The function returns the role name if the user is permitted to the file,
// the permission if the user was shared, and non-nil err if any encountered.
// If no permitted then role found role would be "".
// If user was shared then permission would be non-nil.
func CheckUserFilePermission(ctx context.Context,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	userID string,
	fileID string,
	role ppb.Role) (string, *ppb.PermissionObject, error) {
	if userID == "" {
		return "", nil, fmt.Errorf("userID is required")
	}

	OwnerRole := file.OwnerRole

	// Everyone is permitted to their root, since all actions on root are authenticated,
	// and it's impossible to create a permission for root (aka sharing a user's whole drive).
	if fileID == "" {
		return OwnerRole, nil, nil
	}

	// Get the file's metadata.
	file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		return "", nil, err
	}

	// Check if the owner of the current file is userID, if so then he's permitted.
	if file.GetOwnerID() == userID {
		return OwnerRole, nil, nil
	}

	// Go up the hierarchy searching for a permission for userID to fileID with role.
	// Fetch fileID's parents, each at a time, and check permission to each parent.
	// If reached a parent that userID isn't permitted to then return with error,
	// If reached a parent that userID is permitted to then return true with nil error.
	// If any error encountered then return false and the encountered error.
	currentFile := fileID
	for {
		// If reached the root and didn't find a permission then userID is not permitted to fileID.
		if currentFile == "" {
			return "", nil, nil
		}

		// Check if the user has an existing permission and is permitted to currentFile with the wanted role.
		isPermitted, err := permissionClient.IsPermitted(ctx,
			&ppb.IsPermittedRequest{FileID: currentFile, UserID: userID, Role: role})

		// If an error occurred which is NOT grpc's NotFound error which
		// indicates that the permission doesn't not exist.
		if err != nil && status.Code(err) != codes.NotFound {
			return "", nil, err
		}

		// If no error received and user isn't permitted.
		if !isPermitted.GetPermitted() && err == nil {
			return "", nil, nil
		}

		// If userID is permitted with the wanted role then return the role that the user has for the file.
		if isPermitted.GetPermitted() {
			permission, err := permissionClient.GetPermission(
				ctx,
				&ppb.GetPermissionRequest{
					FileID: currentFile,
					UserID: userID,
				},
			)

			if err != nil {
				return "", nil, err
			}

			return permission.GetRole().String(), permission, nil
		}

		// Get the current file's metadata.
		file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: currentFile})
		if err != nil {
			return "", nil, err
		}

		// Repeat for the file's parent.
		currentFile = file.GetParent()
	}
}

