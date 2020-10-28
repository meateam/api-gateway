package upload

import (
	"fmt"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	"google.golang.org/grpc/status"
)

// deleteUploadOnError removes the upload object in file-service when an error occurs, 
// it returns the quota to it's original size
func (r *Router) deleteUploadOnError(c *gin.Context, err error, key string, bucket string) {
	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	r.deleteUploadOnErrorWithStatus(c, httpStatusCode, err, key, bucket)
}

// deleteUploadOnErrorWithStatus removes the upload object in file service when an error occurs with an http status,
// returns the quota to it's original size
func (r *Router) deleteUploadOnErrorWithStatus(c *gin.Context, status int, err error, key string, bucket string) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	_, deleteErr := r.fileClient.DeleteUploadByKey(c.Request.Context(), &fpb.DeleteUploadByKeyRequest{
		Key: key,
		Bucket: bucket,
	})

	if deleteErr != nil {
		err = fmt.Errorf("%v: %v", err, deleteErr)
	}

	loggermiddleware.LogError(r.logger, c.AbortWithError(status, err))
}
