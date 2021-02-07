package producer

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/meateam/api-gateway/factory"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	prdcr "github.com/meateam/listener-service/proto/producer"
)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "fileID"
)


// Router is a structure that handles producer requests.
type Router struct {
	// PermitClientFactory
	producerClient factory.ProducerClientFactory

	logger          *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	producerConn *grpcPoolTypes.ConnPool,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.producerClient = func() prdcr.ProducerServiceClient {
		return prdcr.NewProducerServiceClient((*producerConn).Conn())
	}

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.POST(fmt.Sprintf("/producer/file/:%s/contentchange", ParamFileID), r.SendContentChange)
	rg.POST(fmt.Sprintf("/producer/file/:%s/permissiondelete", ParamFileID), r.SendPermissionDelete)
}


// SendContentChange - send msg to rabbit queue about content change
func (r *Router) SendContentChange(c *gin.Context) {
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

// 	// Check if the user has the right permission to send rabbit msg for the file
// 	userFilePermission, _, err := file.CheckUserFilePermission(
// 		c.Request.Context(),
// 		r.fileClient(),
// 		r.permissionClient(),
// 		reqUser.ID,
// 		fileID,
// 		ppb.Role_READ,
// 	)
// 	if err != nil && status.Code(err) != codes.NotFound {
// 		r.logger.Errorf("failed get permission with fileId %s, error: %v", fileID, err)
// 	}
// 
// 	if userFilePermission == "" {
// 		r.logger.Errorf("the user doesn't have the permission to change the file %s", fileID)
// 	}

	// Send rabbit msg about content change
	res, err := r.producerClient().SendContentChange(
		c.Request.Context(),
		&prdcr.SendContentChangeRequest{FileID: fileID},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, res)
}

// SendPermissionDelete - send msg to rabbit queue about permission change
func (r *Router) SendPermissionDelete(c *gin.Context) {
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

	res, err := r.producerClient().SendPermissionDelete(
		c.Request.Context(),
		&prdcr.SendPermissionDeleteRequest{FileID: fileID},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, res)
}
