package main

import (
	"fmt"
	"strings"
	"github.com/gin-gonic/gin"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/protos"
	"google.golang.org/grpc"
	"io"
	"net/http"
)

type fileRouter struct {
	downloadClient dpb.DownloadClient
	fileClient     fpb.FileServiceClient
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
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	
	if isUserAllowed == false {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	getFileByIDRequest := &fpb.GetByFileByIDRequest{
		Id: fileID,
	}

	file, err := fr.fileClient.GetFileByID(c, getFileByIDRequest)

	if err != nil {
		if strings.Contains(err.Error(), "File not found") {
			c.Status(http.StatusNotFound)
			return
		}

		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, file)
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
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		
		if isUserAllowed == false {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	filesResp, err := fr.fileClient.GetFilesByFolder(c, &fpb.GetFilesByFolderRequest{OwnerID: reqUser.id, FolderID: filesParent})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, filesResp.GetFiles())
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
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	
	if isUserAllowed == false {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	deleteFileRequest := &fpb.DeleteFileRequest{
		Id: fileID,
	}
	deleteFileResponse, err := fr.fileClient.DeleteFile(c, deleteFileRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
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
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	
	if isUserAllowed == false {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Get the file meta from the file service
	fileMeta, err := fr.fileClient.GetFileByID(c, &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	filename := fileMeta.GetFullName()
	contentType := fileMeta.GetType()
	contentLength := fmt.Sprintf("%d", fileMeta.GetSize())

	downloadRequest := &dpb.DownloadRequest{
		Key:    fileMeta.GetKey(),
		Bucket: fileMeta.GetBucket(),
	}

	stream, err := fr.downloadClient.Download(c, downloadRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
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
			c.AbortWithError(http.StatusInternalServerError, err)
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
	
	isAllowedResp, err := fr.fileClient.IsAllowed(c, &fpb.IsAllowedRequest{
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
