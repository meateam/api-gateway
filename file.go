package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type fileRouter struct {
	downloadClient dpb.DownloadClient
	fileClient     fpb.FileServiceClient
}

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

func (fr *fileRouter) setup(r *gin.Engine, fileConn *grpc.ClientConn, downloadConn *grpc.ClientConn) {
	fr.fileClient = fpb.NewFileServiceClient(fileConn)
	fr.downloadClient = dpb.NewDownloadClient(downloadConn)
	r.GET("/files", fr.getFilesByFolder)
	r.GET("/files/:id", fr.getFileByID)
	r.DELETE("/files/:id", fr.deleteFileByID)

	return
}

func (fr *fileRouter) getFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	alt := c.Query("alt")
	if alt == "media" {
		fr.download(c)
		return
	}
	isUserAllowed, err := fr.userFilePermission(c, fileID)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	if isUserAllowed == false {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	getFileByIDRequest := &fpb.GetByFileByIDRequest{
		Id: fileID,
	}

	file, err := fr.fileClient.GetFileByID(c.Request.Context(), getFileByIDRequest)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	responseFile, err := createGetFileResponse(file)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, responseFile)
	return
}

func (fr *fileRouter) getFilesByFolder(c *gin.Context) {
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filesParent, exists := c.GetQuery("parent")
	if exists == true {
		isUserAllowed, err := fr.userFilePermission(c, filesParent)
		if err != nil {
			c.AbortWithError(int(status.Code(err)), err)
			return
		}
		if isUserAllowed == false {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	filesResp, err := fr.fileClient.GetFilesByFolder(c.Request.Context(), &fpb.GetFilesByFolderRequest{OwnerID: reqUser.id, FolderID: filesParent})
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	files := filesResp.GetFiles()
	responseFiles := make([]*getFileByIDResponse, 0, len(files))
	for _, file := range files {
		responseFile, err := createGetFileResponse(file)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		responseFiles = append(responseFiles, responseFile)
	}

	c.JSON(http.StatusOK, responseFiles)
	return
}

func (fr *fileRouter) deleteFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}
	isUserAllowed, err := fr.userFilePermission(c, fileID)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}
	if isUserAllowed == false {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteFileRequest := &fpb.DeleteFileRequest{
		Id: fileID,
	}
	deleteFileResponse, err := fr.fileClient.DeleteFile(c.Request.Context(), deleteFileRequest)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	c.JSON(http.StatusOK, deleteFileResponse.GetOk())
	return
}

func (fr *fileRouter) download(c *gin.Context) {
	// Get file ID from param.
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	isUserAllowed, err := fr.userFilePermission(c, fileID)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}
	if isUserAllowed == false {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Get the file meta from the file service
	fileMeta, err := fr.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	filename := fileMeta.GetFullName()
	contentType := fileMeta.GetType()
	contentLength := fmt.Sprintf("%d", fileMeta.GetSize())

	downloadRequest := &dpb.DownloadRequest{
		Key:    fileMeta.GetKey(),
		Bucket: fileMeta.GetBucket(),
	}

	span, spanCtx := startSpan(c.Request.Context(), "/download.Download/Download")
	defer span.End()
	stream, err := fr.downloadClient.Download(spanCtx, downloadRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		c.AbortWithError(httpStatusCode, err)
		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", contentLength)

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			c.Status(http.StatusOK)
			// Returns error, need to decide how to handle
			stream.CloseSend()
			return
		}

		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			c.AbortWithError(httpStatusCode, err)
			stream.CloseSend()
			return
		}

		part := chunk.GetFile()
		c.Writer.Write(part)
		c.Writer.Flush()
	}
}

// userFilePermission gets a gin context holding the requesting user and the id of
// the file he's requesting. The function returns (true, nil) if the user is permitted
// to the file, (false, nil) if the user isn't permitted to it, and (false, error) where
// error is non-nil if an error occurred.
func (fr *fileRouter) userFilePermission(c *gin.Context, fileID string) (bool, error) {
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		return false, nil
	}
	isAllowedResp, err := fr.fileClient.IsAllowed(c.Request.Context(), &fpb.IsAllowedRequest{
		FileID: fileID,
		UserID: reqUser.id,
	})

	if err != nil {
		return false, err
	}

	if isAllowedResp.GetAllowed() != true {
		return false, nil
	}

	return true, nil
}

// Creates a file grpc response to http response struct
func createGetFileResponse(file *fpb.File) (*getFileByIDResponse, error) {
	// Get file parent ID, if it doesn't exist check if it's an file object and get its ID.
	fileParentID := file.GetParent()
	if fileParentID == "" {
		fileParentObject := file.GetParentObject()
		if fileParentObject == nil {
			return nil, fmt.Errorf("file parent is invalid")
		}

		fileParentID = fileParentObject.GetId()
	}

	responseFile := &getFileByIDResponse{
		ID:          file.GetId(),
		Name:        file.GetFullName(),
		Type:        file.GetType(),
		Size:        file.GetSize(),
		Description: file.GetDescription(),
		OwnerID:     file.GetOwnerID(),
		Parent:      fileParentID,
		CreatedAt:   file.GetCreatedAt(),
		UpdatedAt:   file.GetUpdatedAt(),
	}

	return responseFile, nil
}
