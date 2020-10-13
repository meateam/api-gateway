package upload

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/status"

	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
)

const (
	// FileIDBody -file id body paramater name
	FileIDBody = "fileId"

	// DestOwnerBody - destination owner paramater name
	DestOwnerBody = "destOwner"
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

	// Check if the copy is for folder or file type
	if file.GetType() == FolderContentType {
		// TODO: implement copy folder
		// (recursive function that calls copyFile or copyFolder)
		// r.CopyFolder()
		return
	}

	// // TODO: implement which kind of copy - large files (more than 5gb) or less
	// // file size in bytes
	// LargeFileSize, err := bytefmt.ToBytes("5G") // TODO: make const

	// if err != nil {
	// 	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	// 	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

	// 	return
	// }

	// if file.GetSize() > LargeFileSize {
	// 	r.CopyLargeFile()
	// }

}

// Move is the request handler for /move/:fileId request.
func (r *Router) Move(c *gin.Context) {
	// Copy and delete
}
