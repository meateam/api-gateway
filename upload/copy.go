package upload

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"

	upb "github.com/meateam/upload-service/proto"
)

const (
	// FileIDBody -file id body paramater name
	FileIDBody = "fileId"

	// DestOwnerBody - destination owner paramater name
	DestOwnerBody = "newOwnerId"

	// LargeFileSize is 5GB
	LargeFileSize = 5 << 30
)

// copyFileRequest is a structure of the json body of copy request.
type copyFileRequest struct {
	FileID     string `json:"fileId"`
	NewOwnerID string `json:"newOwnerId"`
}

// CopySetup initializes its routes under rg.
func (r *Router) CopySetup(rg *gin.RouterGroup) {
	rg.POST("/copy", r.Copy)
	rg.POST("/move", r.Move)
}

// Copy is the request handler for /copy request.
func (r *Router) Copy(c *gin.Context) {
	// Get the user from request
	if reqUser := r.getUserFromContext(c); reqUser == nil {
		return
	}

	// Get the request body
	var reqBody copyFileRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unexpected body format")),
		)

		return
	}

	if reqBody.FileID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", FileIDBody))
		return
	}

	if reqBody.NewOwnerID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", DestOwnerBody))
		return
	}

	//// TODO: Check if the dest owner id is exists
	// userExists, err := r.userClient.GetUserByID(c.Request.Context(), &upb.GetByIDRequest{Id: reqBody.NewOwner})

	// if err != nil {
	// 	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	// 	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
	// 	return
	// }

	// if userExists.GetUser() == nil || userExists.GetUser().GetId() != permission.UserID {
	// 	c.AbortWithStatus(http.StatusBadRequest)
	// 	return
	// }

	isMoving := false

	r.CopyInit(c, reqBody.FileID, reqBody.NewOwnerID, isMoving)
}

// CopyInit - ...
func (r *Router) CopyInit(c *gin.Context, fileID string, newOwnerID string, isMoving bool) {
	reqUser := user.ExtractRequestUser(c)

	// Get the file by id
	file, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// // Check if the owner of the current file isn't the requesting user, if so then he's not permitted to change the ownership
	// if file.GetOwnerID() != reqUser.ID {
	// 	return // TODO: return an error?
	// }

	// Check if user remaining quota is less than the file size
	getFileSizeReq := &fpb.GetFileSizeByIDRequest{Id: fileID, OwnerID: reqUser.ID}
	getFileSizeRes, err := r.fileClient.GetFileSizeByID(c.Request.Context(), getFileSizeReq)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	var newParentID *string = nil

	// Create update in db and allocate quota - with the new owner and parent of the file
	updateObjectReq := &fpb.CreateUploadRequest{
		Bucket:  newOwnerID,
		Name:    file.GetName(),
		OwnerID: newOwnerID,
		Parent:  *newParentID,
		Size:    file.GetSize(),
	}

	_, err = r.fileClient.CreateUpload(c.Request.Context(), updateObjectReq) // TODO: remove the upload later by id and decrease the quota
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	// Check if the file's type is a folder
	if file.GetType() == FolderContentType {
		r.CopyFolder(c, file, newOwnerID, isMoving, nil)
		return
	}

	r.CopyFile(c, fileID, newOwnerID, isMoving, nil)
}

// CopyFile - copy a file object between buckets
func (r *Router) CopyFile(c *gin.Context, fileID string, newOwnerID string, isMoving bool, parentID *string) {
	reqUser := user.ExtractRequestUser(c)

	// Get the file by id
	file, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: fileID},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Check if the owner of the current file isn't the requesting user, if so then he's not permitted to change the ownership
	if file.GetOwnerID() != reqUser.ID {
		return // TODO: return an error?
	}

	// Check if the file's type is a folder
	if file.GetType() == FolderContentType {
		r.CopyFolder(c, file, newOwnerID, isMoving, parentID)
		return
	}

	// If the file size is larger than 5gb, we can't use the copy function
	// and we need to do a multipart upload and delete
	// s3 doesn't allowed to copy objects larger than 5gb between buckets
	if file.GetSize() > LargeFileSize {
		CopyLargeFile(c, file)
		return
	}

	// Generate a new destination key
	keyResp, err := r.fileClient.GenerateKey(c.Request.Context(), &fpb.GenerateKeyRequest{})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	keySrc := file.GetKey()
	keyDest := keyResp.GetKey()

	// Copy object between buckets
	copyObjectReq := &upb.CopyObjectRequest{
		BucketSrc:            file.GetBucket(),
		BucketDest:           newOwnerID, // TODO: check if we need to normalizeCephBucketName
		KeySrc:               keySrc,
		KeyDest:              keyDest,
		IsDeleteSourceObject: isMoving,
	}

	// TODO: add version ID for updating the upload mongo
	copyObjectRes, err := r.uploadClient.CopyObject(c.Request.Context(), copyObjectReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// TODO: add delete upload
	// deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
	// 	UploadID: copyObjectRes.GetCopied(),
	// }

	// r.mu.Lock()
	// defer r.mu.Unlock()
	// _, err = r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	// if err != nil {
	// 	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	// 	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

	// 	return
	// }

	// TODO: change the implmentaion in the file service to see if the owner id is different
	updateFileRequest := &fpb.UpdateFilesRequest{IdList: []string{fileID},
		PartialFile: &fpb.File{
			Key:      keyDest,
			Bucket:   newOwnerID,
			OwnerID:  newOwnerID,
			FileOrId: &fpb.File_Parent{*parentID},
		}}
	updateFilesResponse, err := r.fileClient.UpdateFiles(c.Request.Context(), updateFileRequest)

	if err != nil {
		// TODO: implement delete copy on error
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Only refers to one, because it cannot update more than one
	if len(updateFilesResponse.GetFailedFiles()) != 0 {
		failedFileID := updateFilesResponse.GetFailedFiles()[0]
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error while updating file %s", failedFileID))
		return
	}

	// // TODO: change response
	// c.Header(UploadIDCustomHeader, copyObjectRes.GetCopied())
	// c.Status(http.StatusOK)
}

// Move is the request handler for /move/:fileId request.
func (r *Router) Move(c *gin.Context) {
	// Copy and delete
}

// CopyLargeFile ...
func CopyLargeFile(c *gin.Context, file *fpb.File) {
	// TODO: implement copy CopyLargeFile
	// calls multipart upload file and delete
}

// CopyFolder - recursive function that calls copyFile or copyFolder
// TODO: implement copy folder
func (r *Router) CopyFolder(c *gin.Context, folder *fpb.File, newOwnerID string, isMoving bool, parentID *string) {
	reqUser := user.ExtractRequestUser(c)

	// Get files by folder
	filesResp, err := r.fileClient.GetFilesByFolder(
		c.Request.Context(),
		&fpb.GetFilesByFolderRequest{OwnerID: reqUser.ID, FolderID: folder.GetId()},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// TODO: add lock for a list
	// Copy each descendant file in the folder
	folderID := folder.GetId()
	files := filesResp.GetFiles()
	for _, file := range files {
		r.CopyFile(c, file.GetId(), newOwnerID, isMoving, &folderID)

		// TODO: if failed what to do ??
	}

	updateFileRequest := &fpb.UpdateFilesRequest{IdList: []string{folder.GetId()},
		PartialFile: &fpb.File{
			OwnerID:  newOwnerID,
			FileOrId: &fpb.File_Parent{*parentID},
		}}
	updateFilesResponse, err := r.fileClient.UpdateFiles(c.Request.Context(), updateFileRequest)

	if err != nil {
		// TODO: implement delete copy on error
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Only refers to one, because it cannot update more than one
	if len(updateFilesResponse.GetFailedFiles()) != 0 {
		failedFileID := updateFilesResponse.GetFailedFiles()[0]
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error while updating file %s", failedFileID))
		return
	}

}
