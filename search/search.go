package search

import (
	"fmt"
	"net/http"

	aspb "github.com/MomentumTeam/index-service/search-service/proto/search"
	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	ppb "github.com/meateam/permission-service/proto"
	spb "github.com/meateam/search-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// SearchTermQueryKey is the query key for search request.
	SearchTermQueryKey = "q"
)

// AdvancedSearchRequest is the filter request for advanced search
type AdvancedSearchRequest struct {
	Fields 		aspb.MetaData 	`json:"fields"`
	Amount  	aspb.Amount 	`json:"amount" binding:"required"`
	ExactMatch 	bool 			`json:"exactMatch"`
}

// AdvancedSearchResponse is the response from the advancedSearch
type AdvancedSearchResponse struct {
	File 				*file.GetFileByIDResponse 	`json:"file"`
	HighlightedContent 	string 						`json:"highlightedContent"`
}

// Router is a structure that handles upload requests.
type Router struct {
	// SearchClientFactory
	searchClient factory.SearchClientFactory

	// AdvancedSearchFactory
	advancedSearchClient factory.AdvancedSearchFactory

	// FileClientFactory
	fileClient factory.FileClientFactory

	// PermissionClientFactory
	permissionClient factory.PermissionClientFactory

	logger *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of search Service
// and File Service with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	searchConn *grpcPoolTypes.ConnPool,
	advancedSearchConn *grpcPoolTypes.ConnPool,
	fileConn *grpcPoolTypes.ConnPool,
	permissionConn *grpcPoolTypes.ConnPool,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.searchClient = func() spb.SearchClient {
		return spb.NewSearchClient((*searchConn).Conn())
	}

	r.advancedSearchClient = func() aspb.SearchClient {
		return aspb.NewSearchClient((*advancedSearchConn).Conn())
	}

	r.fileClient = func() fpb.FileServiceClient {
		return fpb.NewFileServiceClient((*fileConn).Conn())
	}

	r.permissionClient = func() ppb.PermissionClient {
		return ppb.NewPermissionClient((*permissionConn).Conn())
	}

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/search", r.Search)
	rg.POST("/search/advanced", r.AdvancedSearch) 
}

// Search is the request handler for /search request.
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

	searchResponse, err := r.searchClient().Search(c.Request.Context(), &spb.SearchRequest{Term: term})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	var responseFiles []*file.GetFileByIDResponse

	for _, id := range searchResponse.GetIds() {
		userFilePermission, foundPermission, err := file.CheckUserFilePermission(
			c.Request.Context(),
			r.fileClient(),
			r.permissionClient(),
			reqUser.ID,
			id,
			ppb.Role_READ,
		)
		if err != nil && status.Code(err) != codes.NotFound {
			r.logger.Errorf("failed get permission with fileId %s, error: %v", id, err)
		}

		if userFilePermission != "" {
			res, err := r.fileClient().GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: id})
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


// AdvancedSearch is the request handler for /search/advanced request.
func (r *Router) AdvancedSearch(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("error extracting user from request")),
		)

		return
	}
	
	// Parsing search request
	var filters AdvancedSearchRequest
	if err := c.Bind(&filters); err != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed parse search query")),
		)

		return
	}
	
	// Call advanced search service
	searchRequest := &aspb.SearchRequest{
		UserID: reqUser.ID,
		ExactMatch: filters.ExactMatch,
		ResultsAmount: &filters.Amount,
		Fields: &filters.Fields,
	}

	// Making a search request
	searchResponse, err := r.advancedSearchClient().Search(c.Request.Context(), searchRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Create response files
	var responseFiles []AdvancedSearchResponse

	for _, result := range searchResponse.GetResults() {
		// Get user permission
		userFilePermission, foundPermission, err := file.CheckUserFilePermission(
			c.Request.Context(),
			r.fileClient(),
			r.permissionClient(),
			reqUser.ID,
			result.GetFileId(),
			ppb.Role_READ,
		)
		if err != nil && status.Code(err) != codes.NotFound {
			r.logger.Errorf("failed get permission with fileId %s, error: %v", result.GetFileId(), err)
		}

		if userFilePermission != "" {
			// Get file object
			res, err := r.fileClient().GetFileByID(
				c.Request.Context(),
				&fpb.GetByFileByIDRequest{Id: result.GetFileId()},
			)
			if err != nil {
				httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
				loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

				return
			}

			responseFiles = append(
				responseFiles,
				AdvancedSearchResponse{
					file.CreateGetFileResponse(res, userFilePermission, foundPermission),
					result.HighlightedContent,
				},
			)
		}
	}

	c.JSON(http.StatusOK, responseFiles)
}
