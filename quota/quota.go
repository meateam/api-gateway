package quota

import (
	"net/http"

	"github.com/gin-gonic/gin"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	qpb "github.com/meateam/file-service/proto/quota"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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
	rg.GET("/users/:id/quota", r.GetOwnerQuota)
}

// GetOwnerQuota is the request handler for GET /users/:id/quota
func (r *Router) GetOwnerQuota(c *gin.Context) {
	ownerID := c.Param("id")

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	allowed := r.isAllowedToGetQuota(c, reqUser.ID, ownerID)
	if !allowed {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	quota, err := r.quotaClient.GetOwnerQuota(
		c.Request.Context(),
		&qpb.GetOwnerQuotaRequest{OwnerID: ownerID},
	)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
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
