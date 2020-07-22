package upload

import (
	"fmt"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	"google.golang.org/grpc/status"
)

// deleteUploadOnError remove the upload object in file service when have error, its returns the quota to origin size
func (r *Router) deleteUploadOnError(c *gin.Context, err error, key string, bucket string) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	_, deleteErr := r.fileClient.DeleteUploadByKey(c.Request.Context(), &fpb.DeleteUploadByKeyRequest{
		Key: key,
		Bucket: bucket,
	})

	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))

	if deleteErr != nil {
		err = fmt.Errorf("%v: %v", err, deleteErr)
	}

	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
}
