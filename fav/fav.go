package fav

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/factory"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"
	"github.com/sirupsen/logrus"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fvpb "github.com/meateam/fav-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"

)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"
)

// Fav is a struct that describes a user's request to a favorite file.
type Fav struct {
	UserID string `json:"userID,omitempty"`
	FileID string `json:"fileID,omitempty"`
}

// Router is a structure that handles favorite requests.
type Router struct {
	favClient       factory.FavClientFactory
	fileClient 		factory.FileClientFactory
	oAuthMiddleware *oauth.Middleware
	logger          *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	favConn *grpcPoolTypes.ConnPool,
	fileConn *grpcPoolTypes.ConnPool,
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

	r.oAuthMiddleware = oAuthMiddleware

	return r
	
}

//Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.POST(fmt.Sprintf("/:%s/fav", ParamFileID), r.CreateFav)
	rg.DELETE(fmt.Sprintf("/:%s/fav", ParamFileID), r.DeleteFav)
	rg.GET("/fav", r.GetAll)

}

// CreateFav creates a favorite for a given file.
// File id is extracted from url params.
func (r *Router) CreateFav(c *gin.Context) {
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

	userID := reqUser.ID

	file, err := r.fileClient().GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID}) 
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	// An app cannot create a permission for a file that does not belong to it.
	// Unless the app is Drive.
	ctxAppID := c.Value(oauth.ContextAppKey).(string)
	if (ctxAppID != file.GetAppID()) && (ctxAppID != oauth.DriveAppID) {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusForbidden, err))
		return
	}

	createReq := &fvpb.CreateFavoriteRequest{FileID: fileID, UserID: userID}
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

// DeleteFav deletes a favorite
// File id is extracted from url params.
func (r *Router) DeleteFav(c *gin.Context) {
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

	userID := reqUser.ID

	deleteRequest := &fvpb.DeleteFavoriteRequest{FileID: fileID, UserID: userID}
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

// GetAll gets all user's favorites
func (r *Router) GetAll(c *gin.Context) {

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	userID := reqUser.ID
	getAllRequest := &fvpb.GetAllFavoritesRequest{UserID: userID}
	favList, err := r.favClient().GetAllFavorites(c.Request.Context(), getAllRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, favList)

}

