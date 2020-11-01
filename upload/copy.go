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

	// UploadBucketCustomHeader ..
	UploadBucketCustomHeader = "uploadBucket"

	// UploadKeyCustomHeader ..
	UploadKeyCustomHeader = "uploadKey"

	// LargeFileSize is 5GB
	LargeFileSize = 5 << 30
)

// copyFileBody is a structure of the json body of copy request.
type copyFileBody struct {
	FileID     string `json:"fileId"`
	NewOwnerID string `json:"newOwnerId"`
}

// copyFileRequest ...
type copyFileRequest struct {
	file       *fpb.File
	newOwnerID string
	isMoving   bool
	parentID   *string
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
	var reqBody copyFileBody
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

	// Check if the owner of the current file isn't the requesting user, if so then he's not permitted to change the ownership
	if file.GetOwnerID() != reqUser.ID {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("You do not have permission to do this operation")))
		return
	}

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
		Size:    getFileSizeRes.GetFileSize(),
	}

	upload, err := r.fileClient.CreateUpload(c.Request.Context(), updateObjectReq) // TODO: remove the upload later by id and decrease the quota and if the copy failed, release the upload quota
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	copyReq := copyFileRequest{
		file:       file,
		newOwnerID: newOwnerID,
		isMoving:   isMoving,
		parentID:   nil,
	}

	// Check if the file's type is a folder
	if file.GetType() == FolderContentType {
		r.CopyFolder(c, copyReq)
	} else {
		r.CopyFile(c, copyReq)
	}

	c.Header(UploadBucketCustomHeader, upload.GetBucket())
	c.Header(UploadKeyCustomHeader, upload.GetKey())
	c.Status(http.StatusOK)
}

// CopyComplete ...
func (r *Router) CopyComplete(c *gin.Context, copyFileRequest copyFileRequest) {
	// TODO: release update quota ?
}

// CopyFile - copy an object between buckets and change owner
func (r *Router) CopyFile(c *gin.Context, copyFileRequest copyFileRequest) {
	file := copyFileRequest.file

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
		BucketDest:           copyFileRequest.newOwnerID, // TODO: check if we need to normalizeCephBucketName
		KeySrc:               keySrc,
		KeyDest:              keyDest,
		IsDeleteSourceObject: copyFileRequest.isMoving,
	}

	// TODO: add version ID for updating the upload mongo
	_, err = r.uploadClient.CopyObject(c.Request.Context(), copyObjectReq)
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

	r.ChangeOwner(c, file.GetId(), &fpb.File{
		Key:      keyDest,
		Bucket:   copyFileRequest.newOwnerID,
		OwnerID:  copyFileRequest.newOwnerID,
		FileOrId: &fpb.File_Parent{*copyFileRequest.parentID},
	})
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
func (r *Router) CopyFolder(c *gin.Context, copyFolderRequest copyFileRequest) {
	reqUser := user.ExtractRequestUser(c)
	folderID := copyFolderRequest.file.GetId()

	// Get descendants by folder id
	descendantsResp, err := r.fileClient.GetDescendantsByID(c.Request.Context(), &fpb.GetDescendantsByIDRequest{Id: folderID})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// TODO: add lock for a list
	descendants := descendantsResp.GetDescendants()

	// Copy each descendant in the folder whose owner is the user that made the request
	for _, descendant := range descendants {
		if descendant.GetFile().GetOwnerID() == reqUser.ID {
			parent := descendant.GetParent().GetId()

			copyFile := copyFileRequest{
				file:       descendant.GetFile(),
				newOwnerID: copyFolderRequest.newOwnerID,
				isMoving:   copyFolderRequest.isMoving,
				parentID:   &parent,
			}

			if descendant.GetFile().GetType() != FolderContentType {
				r.CopyFile(c, copyFile)
			} else {
				r.ChangeOwner(c, descendant.GetFile().GetId(), &fpb.File{
					OwnerID:  copyFile.newOwnerID,
					FileOrId: &fpb.File_Parent{*copyFile.parentID},
				})
			}

			// TODO: if failed what to do ??
		}
	}

	r.ChangeOwner(c, folderID, &fpb.File{
		OwnerID:  copyFolderRequest.newOwnerID,
		FileOrId: &fpb.File_Parent{*copyFolderRequest.parentID},
	})
}

// ChangeOwner ...
// TODO: change it to comatible for file and folder
func (r *Router) ChangeOwner(c *gin.Context, fileID string, partialFile *fpb.File) {
	updateFileRequest := &fpb.UpdateFilesRequest{IdList: []string{fileID}, PartialFile: partialFile}
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
