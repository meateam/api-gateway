package delegation

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	dpb "github.com/meateam/delegation-service/proto/delegation-service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	// ParamUserID is the name of the user id param in URL.
	ParamUserID = "id"

	// ParamPartialName is the name of the partial user name param in URL.
	ParamPartialName = "partial"
)

// Router is a structure that handels delegation requests.
type Router struct {
	delegateClient dpb.DelegationClient
	logger         *logrus.Logger
}

// NewRouter creates a new Router. If logger is non-nil then it will be
// set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	delegateConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.delegateClient = dpb.NewDelegationClient(delegateConn)

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET(fmt.Sprintf("/delegators/:%s", ParamUserID), r.GetUserByID)
	rg.GET("/delegators", r.SearchByName)
}

// GetUserByID is the request handler for GET /users/:id
func (r *Router) GetUserByID(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	userID := c.Param(ParamUserID)
	if userID == "" {
		c.String(http.StatusBadRequest, "id is required")
		return
	}

	getUserByIDRequest := &dpb.GetUserByIDRequest{
		Id: userID,
	}

	user, err := r.delegateClient.GetUserByID(c.Request.Context(), getUserByIDRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// SearchByName is the request handler for GET /users
func (r *Router) SearchByName(c *gin.Context) {
	partialName := c.Query(ParamPartialName)
	if partialName == "" {
		c.String(http.StatusBadRequest, "partial name required")
		return
	}

	findUserByNameRequest := &dpb.FindUserByNameRequest{
		Name: partialName,
	}

	user, err := r.delegateClient.FindUserByName(c.Request.Context(), findUserByNameRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}
