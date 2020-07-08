package upload

import (
	"fmt"
	"net/http"
	"strconv"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/upload-service/proto"
)

const (
	// ParamFileID id to update
	ParamFileID = "id"

	// UpdateFileRole is the role that is required of the authenticated requester to have to be
	// permitted to make the UpdateFile action.
	UpdateFileRole = ppb.Role_WRITE
)

// UpdateSetup initializes its routes under rg.
func (r *Router) UpdateSetup(rg *gin.RouterGroup, checkExternalAdminScope gin.HandlerFunc) {
	rg.PUT("/upload/:"+ParamFileID, checkExternalAdminScope, r.Update)
}

// Update is the request handler for /upload/:fileId request.
// Here it is requesting a new upload for a file update
// Update initiates a resumable upload to update a large file.
func (r *Router) Update(c *gin.Context) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	parentID, _ := c.GetQuery(ParentQueryKey)

	fileID := c.Param(ParamFileID)
	file, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: fileID},
	)

	if role, _ := r.HandleUserFilePermission(c, fileID, UpdateFileRole); role == "" {
		return
	}

	if err != nil {
		r.abortWithError(c, err)
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
		Bucket:  reqUser.Bucket,
		Name:    file.Name,
		OwnerID: reqUser.ID,
		Parent:  parentID,
		Size:    newFileSize,
	})

	if err != nil {
		r.abortWithError(c, err)
		return
	}

	uploadInitReq := &upb.UploadInitRequest{
		Key:         createUpdateResponse.GetKey(),
		Bucket:      createUpdateResponse.GetBucket(),
		ContentType: file.Type,
	}

	resp, err := r.uploadClient.UploadInit(c.Request.Context(), uploadInitReq)
	if err != nil {
		r.abortWithError(c, err)
		return
	}

	_, err = r.fileClient.UpdateUploadID(c.Request.Context(), &fpb.UpdateUploadIDRequest{
		Key:      createUpdateResponse.GetKey(),
		Bucket:   createUpdateResponse.GetBucket(),
		UploadID: resp.GetUploadId(),
	})

	if err != nil {
		r.abortWithError(c, err)
		return
	}

	c.Header(UploadIDCustomHeader, resp.GetUploadId())
	c.Status(http.StatusOK)
}

// UpdateComplete completes a resumable update file upload and update the user quota.
// that too delede te old file content in th s3 storage
func (r *Router) UpdateComplete(c *gin.Context) {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	parentID := c.Query(ParentQueryKey)

	isPermitted := r.isUploadPermittedForUser(c, reqUser.ID, parentID)
	if !isPermitted {
		return
	}

	uploadID, exists := r.getQueryFromContextWithAbort(c, UploadIDQueryKey)
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
	if err != nil {
		r.abortWithError(c, err)
		return
	}

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

	// Locks the action so that no such action will occur at the same time
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err = r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	if err != nil {
		r.abortWithError(c, err)
		return
	}

	parent := createParent(parentID)

	updateFilesResponse, err := r.fileClient.UpdateFiles(c.Request.Context(), &fpb.UpdateFilesRequest{
		IdList: []string{fileID},
		PartialFile: &fpb.File{
			Key:      upload.GetKey(),
			FileOrId: parent,
			Size:     resp.GetContentLength(),
		},
	})

	if err != nil {
		r.deleteUpdateOnError(c, err, oldFile, upload)
		return
	}

	// Only refers to one, because it cannot update more than one
	if len(updateFilesResponse.GetFailedFiles()) > 0 {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error while updating file %s", updateFilesResponse.GetFailedFiles()[0]))
		return
	}

	deleteObjectsResponse, err := r.uploadClient.DeleteObjects(c.Request.Context(), &upb.DeleteObjectsRequest{
		Bucket: upload.Bucket,
		Keys:   []string{oldFile.Key},
	})

	// Only refers to one, because it cannot delete more than one
	if len(deleteObjectsResponse.GetFailed()) > 0 {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error while deleting file %s", deleteObjectsResponse.GetFailed()[0]))
		return
	}

	c.String(http.StatusOK, fileID)
}

// deleteUpdateOnError happens when the metadata is not successfully updated.
// it deletes the new s3 content that has been uploaded.
func (r *Router) deleteUpdateOnError(c *gin.Context, err error, oldFile *fpb.File, upload *fpb.GetUploadByIDResponse) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteObjectsResponse, deleteErr := r.uploadClient.DeleteObjects(c.Request.Context(), &upb.DeleteObjectsRequest{
		Bucket: oldFile.GetBucket(),
		Keys:   []string{upload.GetKey()},
	})

	// Creates an error with all the files that were not updated
	for _, failedFileID := range deleteObjectsResponse.GetFailed() {
		err = fmt.Errorf("%v: failed to delete fileID %v", err, failedFileID)
	}

	if deleteErr != nil {
		err = fmt.Errorf("%v: %v", err, deleteErr)
	}

	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	// This will probably fail because entry here is created when there is a problem with the file service
	_, deleteUploadErr := r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	if deleteUploadErr != nil {
		err = fmt.Errorf("%v: %v", err, deleteUploadErr)
	}

	r.abortWithError(c, err)
}

// createParent creates a parent object using the parentID
func createParent(parentID string) *fpb.File_Parent {
	var parent *fpb.File_Parent
	if parentID == "" {
		parent = &fpb.File_Parent{
			Parent: "null",
		}
	} else {
		parent = &fpb.File_Parent{
			Parent: parentID,
		}
	}
	return parent
}
