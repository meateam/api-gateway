package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/file-service/protos"
	"google.golang.org/grpc"
)

type fileRouter struct {
	client         pb.FileServiceClient
	fileServiceURL string
}

func (fr *fileRouter) setup(r *gin.Engine) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(fr.fileServiceURL, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	fr.client = pb.NewFileServiceClient(conn)
	r.GET("/files", fr.getFilesByFolder)
	r.DELETE("/files/:id", fr.deleteFileByID)

	return conn, nil
}

func (fr *fileRouter) getFilesByFolder(c *gin.Context) {
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filesParent, _ := c.GetQuery("parent")
	filesResp, err := fr.client.GetFilesByFolder(c, &pb.GetFilesByFolderRequest{OwnerID: reqUser.id, FolderID: filesParent})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, filesResp.GetFiles())
	return
}

func (fr *fileRouter) deleteFileByID(c *gin.Context) {
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	deleteFileRequest := &pb.DeleteFileRequest{
		Id: fileID,
	}
	deleteFileResponse, err := fr.client.DeleteFile(c, deleteFileRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, deleteFileResponse.GetOk())
	return
}
