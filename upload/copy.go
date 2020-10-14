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
	DestOwnerBody = "destOwner"

	// LargeFileSize is 5gb
	LargeFileSize = 5368706371
)

// copyFileRequest is a structure of the json body of copy request.
type copyFileRequest struct {
	FileID    string `json:"fileId"` // TODO: smart way not to declare twice the name of the json value
	DestOwner string `json:"destOwner"`
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

	// TODO: add check if the dest owner is exist / valid
	if reqBody.DestOwner == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", DestOwnerBody))
		return
	}

	// Get file by id
	file, err := r.fileClient.GetFileByID(
		c.Request.Context(),
		&fpb.GetByFileByIDRequest{Id: reqBody.FileID},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	r.CopyFile(c, file, reqBody.DestOwner)
}

// CopyFile - copy a file object between buckets
func (r *Router) CopyFile(c *gin.Context, file *fpb.File, bucketDest string) {
	// Check if the file's type is a folder
	if file.GetType() == FolderContentType {
		r.CopyFolder(c, file)
		return
	}

	// If the file size is larger than 5gb, we can't use the copy function
	// and we need to do a multipart upload and delete
	// s3 doesn't allowed to copy objects larger than 5gb between buckets
	if file.GetSize() > LargeFileSize {
		r.CopyLargeFile(c, file)
		return
	}

	// Generate new destination key
	keyResp, err := r.fileClient.GenerateKey(c.Request.Context(), &fpb.GenerateKeyRequest{})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	reqUser := user.ExtractRequestUser(c)
	keySrc := file.GetId()
	keyDest := keyResp.GetKey()

	// Copy object between buckets
	// TODO: add lock mu
	copyObjectReq := &upb.CopyObjectRequest{
		BucketSrc:            file.GetBucket(),
		BucketDest:           bucketDest,
		KeySrc:               keySrc,
		KeyDest:              keyDest,
		IsDeleteSourceObject: false, // TODO: change to parameter for move function
	}

	_, err = r.uploadClient.CopyObject(c.Request.Context(), copyObjectReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	_, err = r.fileClient.CreateUpdate(c.Request.Context(), &fpb.CreateUploadRequest{
		Bucket:  bucketDest,
		Name:    file.GetName(),
		OwnerID: bucketDest,
		Parent:  "", // TODO: change parent id
		Size:    file.GetSize(),
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	// Delete file permission
	deleteFilePermissionsReq := &ppb.DeleteFilePermissionsRequest{FileID: file.GetId()}

	if _, err := r.permissionClient.DeleteFilePermissions(c.Request.Context(), deleteFilePermissionsReq); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Add file permissions
	newPermission := ppb.PermissionObject{
		FileID:  keySrc,
		UserID:  bucketDest,
		Role:    ppb.Role_WRITE,
		Creator: reqUser.ID, // TODO: change to gin req user?
	}

	err = filegw.CreatePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID, // TODO: change to gin req user?
		newPermission,
	)

	// // TODO: change response
	// c.Header(UploadIDCustomHeader, copyObjectRes.GetCopied())
	// c.Status(http.StatusOK)
}

// CopyLargeFile ...
func (r *Router) CopyLargeFile(c *gin.Context, file *fpb.File) {
	// TODO: implement copy CopyLargeFile
	// calls multipart upload file and delete
}

// CopyFolder ...
func (r *Router) CopyFolder(c *gin.Context, folder *fpb.File) {
	// TODO: implement copy folder
	// (recursive function that calls copyFile or copyFolder)
}

// Move is the request handler for /move/:fileId request.
func (r *Router) Move(c *gin.Context) {
	// Copy and delete
}
