package upload

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	filegw "github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/upload-service/proto"
)

const (
	// FileIDBody -file id body paramater name
	FileIDBody = "fileId"

	// DestOwnerBody - destination owner paramater name
	DestOwnerBody = "newOwner"

	// LargeFileSize is 5GB
	LargeFileSize = 5 << 30
)

// copyFileRequest is a structure of the json body of copy request.
type copyFileRequest struct {
	FileID   string `json:"fileId"` // TODO: smart way not to declare twice the name of the json value
	NewOwner string `json:"newOwner"`
}

// CopySetup initializes its routes under rg.
func (r *Router) CopySetup(rg *gin.RouterGroup) {
	rg.POST("/copy", r.Copy)
	rg.POST("/move", r.Move)
}

// Copy is the request handler for /copy request.
func (r *Router) Copy(c *gin.Context) {
	// Get the user from request
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return
	}

	// Get the body from request
	var reqBody copyFileRequest
	if err := c.BindJSON(&reqBody); err != nil {
		c.String(http.StatusBadRequest, "invalid request body parameters")
		return
	}

	if reqBody.FileID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", FileIDBody))
		return
	}

	// TODO: add check if the dest owner id is exist
	if reqBody.NewOwner == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", DestOwnerBody))
		return
	}

	isMoving := false
	parentID := "" // TODO: change to null

	r.CopyFile(c, reqBody.FileID, reqBody.NewOwner, isMoving, parentID)
}

// CopyFile - copy a file object between buckets
// TODO: change is moving and parent id to optional arguments
func (r *Router) CopyFile(c *gin.Context, fileID string, newOwner string, isMoving bool, parentID string) {
	// Get file by id
	file, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: fileID},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// If parentId is set it means that this file is the root of the copy, so the parent must be null
	// TODO: check if the parentId is set. if it doesn't the parent id is the the same

	// Check if the file's type is a folder
	if file.GetType() == FolderContentType {
		CopyFolder(c, file, newOwner)
		return
	}

	// TODO: check if the quota of the dest bucket is less than the object size

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

	reqUser := user.ExtractRequestUser(c)
	keySrc := file.GetKey()
	keyDest := keyResp.GetKey()

	// Copy object between buckets
	// TODO: add lock mu
	copyObjectReq := &upb.CopyObjectRequest{
		BucketSrc:            file.GetBucket(),
		BucketDest:           newOwner, // TODO: check if we need to normalizeCephBucketName
		KeySrc:               keySrc,
		KeyDest:              keyDest,
		IsDeleteSourceObject: isMoving, // TODO: change to parameter for move function
	}

	_, err = r.uploadClient.CopyObject(c.Request.Context(), copyObjectReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Create update in db - change owner and parent of the file
	updateObjectReq := &fpb.CreateUploadRequest{
		Bucket:  newOwner,
		Name:    file.GetName(),
		OwnerID: newOwner,
		Parent:  parentID, // TODO: change parent id
		Size:    file.GetSize(),
	}

	if _, err = r.fileClient.CreateUpdate(c.Request.Context(), updateObjectReq); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	// Add file permissions
	newPermission := ppb.PermissionObject{
		FileID:  keySrc,
		UserID:  reqUser.ID,
		Role:    ppb.Role_WRITE,
		Creator: reqUser.ID,
	}

	err = filegw.CreatePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		newPermission,
	)

	if err != nil {
		// TODO: do something...
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

// CopyFolder ...
func CopyFolder(c *gin.Context, folder *fpb.File, newOwner string) {
	// TODO: implement copy folder
	// (recursive function that calls copyFile or copyFolder)

	reqUser := user.ExtractRequestUser(c)
	sizeFolder := GetFolderSize(folder.GetId())

	// TODO: check if user remaining quota is less than the folder size

	// TODO: implement a a folder move
	// Create update in db - change owner and parent of the file
	updateObjectReq := &fpb.CreateUploadRequest{
		Bucket:  newOwner,
		Name:    folder.GetName(),
		OwnerID: newOwner,
		Parent:  parentID, // TODO: change parent id
		Size:    folder.GetSize(),
	}

	if _, err = r.fileClient.CreateUpdate(c.Request.Context(), updateObjectReq); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}
	// TODO: add lock for a list

}

// GetFolderSize ...
func GetFolderSize(folderID string) int64 {
	// TODO: add check folder size
	return 0
}
