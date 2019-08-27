package quota

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	qpb "github.com/meateam/file-service/proto/quota"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	// ParamFileParent is a constant for file parent parameter in a request
	ParamFileParent = "parent"

	// ParamFileName is a constant for file name parameter in a request
	ParamFileName = "name"

	// ParamFileType is a constant for file type parameter in a request
	ParamFileType = "type"

	// ParamFileDescription is a constant for file description parameter in a request
	ParamFileDescription = "description"

	// ParamFileSize is a constant for file size parameter in a request
	ParamFileSize = "size"

	// ParamFileCreatedAt is a constant for file created at parameter in a request
	ParamFileCreatedAt = "createdAt"

	// ParamFileUpdatedAt is a constant for file updated at parameter in a request
	ParamFileUpdatedAt = "updatedAt"
)

// Router is a structure that handles upload requests.
type Router struct {
	quotaClient qpb.QuotaServiceClient
	logger      *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of File Service
// and Download Service with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	fileConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.quotaClient = qpb.NewQuotaServiceClient(fileConn)

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

// Extracts parameters from request query to a map, non-existing parameter has a value of ""
func queryParamsToMap(c *gin.Context, paramNames ...string) map[string]string {
	paramMap := make(map[string]string)
	for _, paramName := range paramNames {
		param, exists := c.GetQuery(paramName)
		if exists {
			paramMap[paramName] = param
		} else {
			paramMap[paramName] = ""
		}
	}
	return paramMap
}

// Converts a string to int64, 0 is returned on failure
func stringToInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		n = 0
	}
	return n
}
