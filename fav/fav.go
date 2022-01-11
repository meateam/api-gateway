package fav

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/api-gateway/utils"
	fvpb "github.com/meateam/fav-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
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
	rg.POST(("/fav/:id"), r.CreateFav)
	rg.DELETE(("/fav/:id"), r.DeleteFav)
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

	if role, _ := utils.HandleUserFilePermission(r.fileClient(), r.permissionClient(), c, fileID, CreateFavFileByIDRole); role == "" {
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


	if role, _ := utils.HandleUserFilePermission(r.fileClient(), r.permissionClient(), c, fileID, DeleteFavFileByIDRole); role == "" {
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
