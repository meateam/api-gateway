package upload

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	upb "github.com/meateam/upload-service/proto"
	"net/http"
)

const (
	// File id to update
	ParamID = "id"
)

// UpdateSetup initializes its routes under rg.
func (r *Router) UpdateSetup(rg *gin.RouterGroup, checkExternalAdminScope gin.HandlerFunc) {
	rg.PUT("/upload/:"+ParamID, checkExternalAdminScope, r.Update)
}

// Update is the request handler for /upload/:fileId request.
// Here it is requesting a new upload for a file update
func (r *Router) Update(c *gin.Context) {
	_, success := r.isUserFromContext(c)
	if !success {
		return
	}
	_, exists := c.GetQuery(UploadTypeQueryKey)
	if !exists {
		r.UpdateInit(c)
		return
	}
}

// UpdateComplete completes a resumable update file upload and update the user quota.
// that too delede te old file content in th s3 storage
func (r *Router) UpdateComplete(c *gin.Context) {
	reqUser, success := r.isUserFromContext(c)
	if !success {
		return
	}

	parentQuery := c.Query(ParentQueryKey)

	isPermitted := r.isUploadPermittedForUser(c, reqUser.ID, parentQuery)
	if !isPermitted {
		return
	}

	uploadID, exists := r.isQueryInContext(c, UploadIDQueryKey)
	if !exists {
		return
	}

	upload, err := r.fileClient.GetUploadByID(c.Request.Context(), &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		r.abortWithError(c, err)
		return
	}

	fileID := upload.GetFileID()
	oldFile, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: fileID},
	)

	uploadCompleteRequest := &upb.UploadCompleteRequest{
		UploadId: uploadID,
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
	}

	resp, err := r.uploadClient.UploadComplete(c.Request.Context(), uploadCompleteRequest)
	if err != nil {
		r.abortWithError(c, err)
		return
	}

	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	_, err = r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	if err != nil {
		r.abortWithError(c, err)
		return
	}

	parent := createParent(parentQuery)

	updateFilesResponse, err := r.fileClient.UpdateFiles(c.Request.Context(), &fpb.UpdateFilesRequest{
		IdList: []string{fileID},
		PartialFile: &fpb.File{
			Key:      upload.GetKey(),
			FileOrId: parent,
			Size:     resp.GetContentLength(),
		},
	})

	if err != nil {
		r.deleteUpdateOnError(c, err, oldFile, upload.GetKey())
		return
	}

	for _, failedFile := range updateFilesResponse.GetFailedFiles() {
		err := errors.New(failedFile.GetError())
		r.abortWithError(c, err)
		return
	}

	deleteObjectsResponse, err := r.uploadClient.DeleteObjects(c.Request.Context(), &upb.DeleteObjectsRequest{
		Bucket: upload.Bucket,
		Keys:   []string{oldFile.Key},
	})

	for _, failedFile := range deleteObjectsResponse.GetFailed() {
		err := errors.New(failedFile)
		r.abortWithError(c, err)
		return
	}

	c.String(http.StatusOK, fileID)
}

// deleteUpdateOnError This happens when the metadata is not successfully updated, it deletes the new content that has been uploaded
func (r *Router) deleteUpdateOnError(c *gin.Context, err error, oldFile *fpb.File, newFileKey string) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteObjectsResponse, deleteErr := r.uploadClient.DeleteObjects(c.Request.Context(), &upb.DeleteObjectsRequest{
		Bucket: oldFile.GetBucket(),
		Keys:   []string{newFileKey},
	})

	for _, failedFile := range deleteObjectsResponse.GetFailed() {
		err = fmt.Errorf("%v: %v", err, failedFile)
	}

	if deleteErr != nil {
		err = fmt.Errorf("%v: %v", err, deleteErr)
	}

	r.abortWithError(c, err)
}

func createParent(parentQuery string) *fpb.File_Parent {
	var parent *fpb.File_Parent
	if parentQuery == "" {
		parent = &fpb.File_Parent{
			Parent: "null",
		}
	} else {
		parent = &fpb.File_Parent{
			Parent: parentQuery,
		}
	}
	return parent
}
