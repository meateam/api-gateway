package quota

import (
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	qpb "github.com/meateam/file-service/proto/quota"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Router is a structure that handles quota related requests.
type Router struct {
	quotaClient qpb.QuotaServiceClient
	logger      *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	quotaConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.quotaClient = qpb.NewQuotaServiceClient(quotaConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/user/quota", r.GetOwnerQuota)
	rg.GET("/users/:id/quota", r.GetQuotaByID)
}

// GetOwnerQuota is the request handler for GET /user/quota
func (r *Router) GetOwnerQuota(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	r.handleGetQuota(c, reqUser.ID, reqUser.ID)
}

// GetQuotaByID is the request handler for GET /users/:id/quota
func (r *Router) GetQuotaByID(c *gin.Context) {
	ownerID := c.Param("id")

	if ownerID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	r.handleGetQuota(c, reqUser.ID, ownerID)
}

// handleGetQuota handles the request to the quota Service and the response to the client.
func (r *Router) handleGetQuota(c *gin.Context, requesterID string, ownerID string) {
	allowed := r.isAllowedToGetQuota(c, requesterID, ownerID)
	if !allowed {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	quota, err := r.quotaClient.GetOwnerQuota(
		c.Request.Context(),
		&qpb.GetOwnerQuotaRequest{OwnerID: ownerID},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, quota)
}

// IsAllowedToGetQuota is used to check if the user is allowed to get another user's quota
func (r *Router) isAllowedToGetQuota(c *gin.Context, reqUserID string, ownerID string) bool {
	res, err := r.quotaClient.IsAllowedToGetQuota(
		c.Request.Context(),
		&qpb.IsAllowedToGetQuotaRequest{RequestingUser: reqUserID, OwnerID: ownerID},
	)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
		return false
	}
	return res.GetAllowed()
}
