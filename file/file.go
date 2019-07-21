package file

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Router is a structure that handles upload requests.
type Router struct {
	downloadClient dpb.DownloadClient
	fileClient     fpb.FileServiceClient
	logger         *logrus.Logger
}

// getFileByIDResponse is a structure used for parsing fpb.File to a json file metadata response.
type getFileByIDResponse struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Description string `json:"description,omitempty"`
	OwnerID     string `json:"ownerId,omitempty"`
	Parent      string `json:"parent,omitempty"`
	CreatedAt   int64  `json:"createdAt,omitempty"`
	UpdatedAt   int64  `json:"updatedAt,omitempty"`
}

// NewRouter creates a new Router, and initializes clients of File Service
// and Download Service with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(fileConn *grpc.ClientConn, downloadConn *grpc.ClientConn, logger *logrus.Logger) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.fileClient = fpb.NewFileServiceClient(fileConn)
	r.downloadClient = dpb.NewDownloadClient(downloadConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/files", r.GetFilesByFolder)
	rg.GET("/files/:id", r.GetFileByID)
	rg.DELETE("/files/:id", r.DeleteFileByID)
}

// GetFileByID is the request handler for GET /files/:id
func (r *Router) GetFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	alt := c.Query("alt")
	if alt == "media" {
		r.Download(c)
		return
	}

	isUserAllowed := r.HandleUserFilePermission(c, fileID)
	if !isUserAllowed {
		return
	}

	getFileByIDRequest := &fpb.GetByFileByIDRequest{
		Id: fileID,
	}

	file, err := r.fileClient.GetFileByID(c.Request.Context(), getFileByIDRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	responseFile, err := createGetFileResponse(file)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, responseFile)
}

// GetFilesByFolder is the request handler for GET /files request.
func (r *Router) GetFilesByFolder(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filesParent, exists := c.GetQuery("parent")
	if exists {
		isUserAllowed := r.HandleUserFilePermission(c, filesParent)
		if !isUserAllowed {
			return
		}
	}

	filesResp, err := r.fileClient.GetFilesByFolder(
		c.Request.Context(),
		&fpb.GetFilesByFolderRequest{OwnerID: reqUser.ID, FolderID: filesParent},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	files := filesResp.GetFiles()
	responseFiles := make([]*getFileByIDResponse, 0, len(files))
	for _, file := range files {
		responseFile, err := createGetFileResponse(file)
		if err != nil {
			loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
			return
		}

		responseFiles = append(responseFiles, responseFile)
	}

	c.JSON(http.StatusOK, responseFiles)
}

// DeleteFileByID is the request handler for DELETE /files/:id request.
func (r *Router) DeleteFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	isUserAllowed := r.HandleUserFilePermission(c, fileID)
	if !isUserAllowed {
		return
	}

	deleteFileRequest := &fpb.DeleteFileRequest{
		Id: fileID,
	}
	deleteFileResponse, err := r.fileClient.DeleteFile(c.Request.Context(), deleteFileRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, deleteFileResponse.GetOk())
}

// Download is the request handler for /files/:id?alt=media request.
func (r *Router) Download(c *gin.Context) {
	// Get file ID from param.
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	isUserAllowed := r.HandleUserFilePermission(c, fileID)
	if !isUserAllowed {
		return
	}

	// Get the file meta from the file service
	fileMeta, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	filename := fileMeta.GetName()
	contentType := fileMeta.GetType()
	contentLength := fmt.Sprintf("%d", fileMeta.GetSize())

	downloadRequest := &dpb.DownloadRequest{
		Key:    fileMeta.GetKey(),
		Bucket: fileMeta.GetBucket(),
	}

	span, spanCtx := loggermiddleware.StartSpan(c.Request.Context(), "/download.Download/Download")
	defer span.End()

	stream, err := r.downloadClient.Download(spanCtx, downloadRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", contentLength)

	loggermiddleware.LogError(r.logger, HandleStream(c, stream))
}

// HandleStream streams the file bytes from stream to c.
func HandleStream(c *gin.Context, stream dpb.Download_DownloadClient) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			c.Status(http.StatusOK)

			// Returns error, need to decide how to handle
			if err := stream.CloseSend(); err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			if err := c.AbortWithError(httpStatusCode, err); err != nil {
				return err
			}

			if err := stream.CloseSend(); err != nil {
				return err
			}

			return nil
		}

		part := chunk.GetFile()
		if _, err := c.Writer.Write(part); err != nil {
			return err
		}
		c.Writer.Flush()
	}
}

// userFilePermission gets a gin context holding the requesting user and the id of
// the file he's requesting. The function returns (true, nil) if the user is permitted
// to the file, (false, nil) if the user isn't permitted to it, and (false, error) where
// error is non-nil if an error occurred when calling FileServiceClient.IsAllowed.
func (r *Router) userFilePermission(c *gin.Context, fileID string) (bool, error) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		return false, nil
	}
	isAllowedResp, err := r.fileClient.IsAllowed(c.Request.Context(), &fpb.IsAllowedRequest{
		FileID: fileID,
		UserID: reqUser.ID,
	})

	if err != nil {
		return false, err
	}

	if !isAllowedResp.GetAllowed() {
		return false, nil
	}

	return true, nil
}

// HandleUserFilePermission gets a gin context and the id of the requested file.
// The function returns true if the user is permitted to operate on the file.
// The function returns false if the user isn't permitted to operate on it,
// The function also returns false if error if error occurred on r.userFilePermission
// and also log the error.
// It also handles error cases and Unauthorized operations by aborting with error/status.
func (r *Router) HandleUserFilePermission(c *gin.Context, fileID string) bool {
	isUserAllowed, err := r.userFilePermission(c, fileID)

	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(int(status.Code(err)), err))
		return false
	}

	if !isUserAllowed {
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	return true
}

// createGetFileResponse Creates a file grpc response to http response struct
func createGetFileResponse(file *fpb.File) (*getFileByIDResponse, error) {
	// Get file parent ID, if it doesn't exist check if it's an file object and get its ID.
	responseFile := &getFileByIDResponse{
		ID:          file.GetId(),
		Name:        file.GetName(),
		Type:        file.GetType(),
		Size:        file.GetSize(),
		Description: file.GetDescription(),
		OwnerID:     file.GetOwnerID(),
		Parent:      file.GetParent(),
		CreatedAt:   file.GetCreatedAt(),
		UpdatedAt:   file.GetUpdatedAt(),
	}

	// If file contains parent object instead of its id.
	fileParentObject := file.GetParentObject()
	if fileParentObject != nil {
		responseFile.Parent = fileParentObject.GetId()
	}

	return responseFile, nil
}
