package quotaapproval

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	qapb "github.com/meateam/quota-approval-service/proto/quotaApproval"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	// PENDING is a status code for pending quota approval request
	PENDING = "REQUEST_PENDING_APPROVAL"

	// APPROVED is a status code for approved quota approval request
	APPROVED = "REQUEST_APPROVED"

	// DENIED is a status code for denied quota approval request
	DENIED = "REQUEST_NOT_APPROVED"
)

// Router is a structure that handles quota-approval related requests.
type Router struct {
	quotaApprovalClient qapb.QuotaApprovalClient
	logger              *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	quotaApprovalConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.quotaApprovalClient = qapb.NewQuotaApprovalClient(quotaApprovalConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/quota/approval/:createdBy/:approvableBy", r.GetQuotasApprovals)
	rg.GET("/quota/approval/:id", r.GetQuotaApprovalByID)
	rg.PUT("/quota/approval/:id/:status", r.UpdateQuotaApproval)
	rg.POST("/quota/approval/:size/:info", r.CreateQuotaApproval)
}

// GetQuotasApprovals is the request handler for GET /quota/approval/:createdBy/:approvableBy
func (r *Router) GetQuotasApprovals(c *gin.Context) {
	createdBy := c.Param("createdBy")
	approvableBy := c.Param("approvableBy")

	if createdBy == "" || approvableBy == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	quotaApprovals, err := r.quotaApprovalClient.GetQuotasApprovals(
		c.Request.Context(),
		&qapb.GetQuotasApprovalsRequest{CreatedBy: createdBy, ApprovableBy: approvableBy},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, quotaApprovals)
}

// GetQuotaApprovalByID is the request handler for GET /quota/approval/:id
func (r *Router) GetQuotaApprovalByID(c *gin.Context) {
	approvalRequestID := c.Param("id")

	if approvalRequestID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	quotaApproval, err := r.quotaApprovalClient.GetQuotaApprovalByID(
		c.Request.Context(),
		&qapb.GetQuotaApprovalByIDRequest{Id: approvalRequestID},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, quotaApproval)
}

// UpdateQuotaApproval is the request handler for PUT /quota/approval/:id/:status
func (r *Router) UpdateQuotaApproval(c *gin.Context) {
	approvalRequestID := c.Param("id")
	approvalRequestStatus := c.Param("status")

	if approvalRequestID == "" || approvalRequestStatus == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if approvalRequestStatus != APPROVED && approvalRequestStatus != PENDING && approvalRequestStatus != DENIED {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	modifiedBy := reqUser.ID

	updatedQuotaApproval, err := r.quotaApprovalClient.UpdateQuotaApproval(
		c.Request.Context(),
		&qapb.UpdateQuotaApprovalRequest{Id: approvalRequestID, ModifiedBy: modifiedBy, Status: approvalRequestStatus},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, updatedQuotaApproval)
}

// CreateQuotaApproval is the request handler for POST /quota/approval/:size/:info
func (r *Router) CreateQuotaApproval(c *gin.Context) {
	approvalRequestSize := stringToInt64(c.Param("size"))
	approvalRequestInfo := c.Param("info")

	if approvalRequestSize == 0 || approvalRequestInfo == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	from := reqUser.ID
	modifiedBy := reqUser.ID

	createdQuotaApproval, err := r.quotaApprovalClient.CreateQuotaApproval(
		c.Request.Context(),
		&qapb.CreateQuotaApprovalRequest{From: from, ModifiedBy: modifiedBy, Size: approvalRequestSize, Info: approvalRequestInfo},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, createdQuotaApproval)
}

// Converts a string to int64, 0 is returned on failure
func stringToInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		n = 0
	}
	return n
}
