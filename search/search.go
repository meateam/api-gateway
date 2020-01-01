package search

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	spb "github.com/meateam/search-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// SearchTermQueryKey is the query key for search request.
	SearchTermQueryKey = "q"
)

// Router is a structure that handles upload requests.
type Router struct {
	searchClient     spb.SearchClient
	fileClient       fpb.FileServiceClient
	permissionClient ppb.PermissionClient
	logger           *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of search Service
// and File Service with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	searchConn *grpc.ClientConn,
	fileConn *grpc.ClientConn,
	permissionConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.searchClient = spb.NewSearchClient(searchConn)
	r.fileClient = fpb.NewFileServiceClient(fileConn)
	r.permissionClient = ppb.NewPermissionClient(permissionConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/search", r.Search)
}

// Search is the request handler for /upload request.
func (r *Router) Search(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("error extracting user from request")),
		)

		return
	}

	term, exists := c.GetQuery(SearchTermQueryKey)
	if !exists {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("missing search term")),
		)

		return
	}

	searchResponse, err := r.searchClient.Search(c.Request.Context(), &spb.SearchRequest{Term: term})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	var responseFiles []*file.GetFileByIDResponse

	for _, id := range searchResponse.GetIds() {
		userFilePermission, foundPermission, err := file.CheckUserFilePermission(
			c.Request.Context(),
			r.fileClient,
			r.permissionClient,
			reqUser.ID,
			id,
			ppb.Role_READ,
		)
		if err != nil && status.Code(err) != codes.NotFound {
			r.logger.Errorf("failed get permission with fileId %s, error: %v", id, err)
		}

		if userFilePermission != "" {
			res, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: id})
			if err != nil {
				httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
				loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

				return
			}

			responseFiles = append(
				responseFiles, file.CreateGetFileResponse(res, userFilePermission, foundPermission))
		}
	}

	c.JSON(http.StatusOK, responseFiles)
}
