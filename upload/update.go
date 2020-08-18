package upload

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/upload-service/proto"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID id to update
	ParamFileID = "id"

	// UpdateFileRole is the role that is required of the authenticated requester to have to be
	// permitted to make the UpdateFile action.
	UpdateFileRole = ppb.Role_WRITE
)

// UpdateSetup initializes its routes under rg.
func (r *Router) UpdateSetup(rg *gin.RouterGroup) {
	rg.PUT("/upload/:"+ParamFileID, r.Update)
}

// Update is the request handler for /upload/:fileId request.
// Here it is requesting a new upload for a file update
// Update initiates a resumable upload to update a large file.
func (r *Router) Update(c *gin.Context) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	fileID := c.Param(ParamFileID)
	file, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: fileID},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	if hasPermission := r.HandleUserFilePermission(c, fileID, UpdateFileRole); !hasPermission {
		return
	}

	newFileSize, err := strconv.ParseInt(c.Request.Header.Get(ContentLengthCustomHeader), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is invalid", ContentLengthCustomHeader))
		return
	}

	if newFileSize < 0 {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is must be positive", ContentLengthCustomHeader))
		return
	}

	createUpdateResponse, err := r.fileClient.CreateUpdate(c.Request.Context(), &fpb.CreateUploadRequest{
		Bucket:  file.GetBucket(),
		Name:    file.GetName(),
		OwnerID: file.GetOwnerID(),
		Parent:  file.GetParent(),
		Size:    newFileSize,
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	uploadInitReq := &upb.UploadInitRequest{
		Key:         createUpdateResponse.GetKey(),
		Bucket:      createUpdateResponse.GetBucket(),
		ContentType: file.Type,
	}

	resp, err := r.uploadClient.UploadInit(c.Request.Context(), uploadInitReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	_, err = r.fileClient.UpdateUploadID(c.Request.Context(), &fpb.UpdateUploadIDRequest{
		Key:      createUpdateResponse.GetKey(),
		Bucket:   createUpdateResponse.GetBucket(),
		UploadID: resp.GetUploadId(),
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.Header(UploadIDCustomHeader, resp.GetUploadId())
	c.Status(http.StatusOK)
}

// UpdateComplete completes a resumable update-file upload and updates the user quota, and deletes the old file's content.
func (r *Router) UpdateComplete(c *gin.Context) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	uploadID, exists := r.getQueryFromContext(c, UploadIDQueryKey)
	if !exists {
		return
	}

	upload, err := r.fileClient.GetUploadByID(c.Request.Context(), &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	fileID := upload.GetFileID()
	oldFile, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: fileID},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	uploadCompleteRequest := &upb.UploadCompleteRequest{
		UploadId: uploadID,
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
	}

	resp, err := r.uploadClient.UploadComplete(c.Request.Context(), uploadCompleteRequest)
	if err != nil {
		r.deleteUpdateOnError(c, err, upload)
		return
	}

	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	// Locks the action so that no such action will occur at the same time
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err = r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	updateFilesResponse, err := r.fileClient.UpdateFiles(c.Request.Context(), &fpb.UpdateFilesRequest{
		IdList: []string{fileID},
		PartialFile: &fpb.File{
			Key:  upload.GetKey(),
			Size: resp.GetContentLength(),
		},
	})

	if err != nil {
		r.deleteUpdateOnError(c, err, upload)
		return
	}

	// Only refers to one, because it cannot update more than one
	if len(updateFilesResponse.GetFailedFiles()) != 0 {
		failedFileID := updateFilesResponse.GetFailedFiles()[0]
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error while updating file %s", failedFileID))
		return
	}

	deleteObjectsResponse, err := r.uploadClient.DeleteObjects(c.Request.Context(), &upb.DeleteObjectsRequest{
		Bucket: upload.Bucket,
		Keys:   []string{oldFile.Key},
	})

	// Only refers to one, because it cannot delete more than one
	if len(deleteObjectsResponse.GetFailed()) != 0 {
		failedFileID := deleteObjectsResponse.GetFailed()[0]
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error while deleting file %s", failedFileID))
		return
	}

	c.String(http.StatusOK, fileID)
}

// deleteUpdateOnError handles an error in the update process after the new-file's content has been uploaded.
// It deletes the new-file's content.
func (r *Router) deleteUpdateOnError(c *gin.Context, err error, upload *fpb.GetUploadByIDResponse) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	deleteObjectsResponse, deleteErr := r.uploadClient.DeleteObjects(c.Request.Context(), &upb.DeleteObjectsRequest{
		Bucket: upload.GetBucket(),
		Keys:   []string{upload.GetKey()},
	})

	// Creates an error with the file that were not updated
	if len(deleteObjectsResponse.GetFailed()) != 0 {
		failedFileID := deleteObjectsResponse.GetFailed()[0]
		err = fmt.Errorf("%v: failed to delete fileID %v", err, failedFileID)
	}

	if deleteErr != nil {
		err = fmt.Errorf("%v: %v", err, deleteErr)
	}

	// This will probably fail because entry here is created when there is a problem with the file service
	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	_, deleteUploadErr := r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	if deleteUploadErr != nil {
		err = fmt.Errorf("%v: fail to delete upload %v", err, deleteUploadErr)
	}

	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
}
