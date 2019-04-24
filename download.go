package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/protos"
	"google.golang.org/grpc"
)

type downloadRouter struct {
	client             pb.DownloadClient
	fileClient         fpb.FileServiceClient
	downloadServiceURL string
}

func (dr *downloadRouter) setup(r *gin.Engine, fileConn *grpc.ClientConn) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(dr.downloadServiceURL, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	dr.client = pb.NewDownloadClient(conn)

	if fileConn == nil {
		return nil, fmt.Errorf("file service connection is nil")
	}
	dr.fileClient = fpb.NewFileServiceClient(fileConn)

	r.GET("/download/:id", dr.download)

	return conn, nil
}

func (dr *downloadRouter) download(c *gin.Context) {
	// Get ID from param.
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "id param is required")
		return
	}

	// Get the file meta from the file service
	fileMeta, err := dr.fileClient.GetFileByID(c, &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	isAllowedResp, err := dr.fileClient.IsAllowed(c, &fpb.IsAllowedRequest{
		FileID: fileID,
		UserID: reqUser.id,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if isAllowedResp.GetAllowed() != true {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filename := fileMeta.GetFullName()
	contentType := fileMeta.GetType()
	contentLength := fmt.Sprintf("%d", fileMeta.GetSize())

	downloadRequest := &pb.DownloadRequest{
		Key:    fileMeta.GetKey(),
		Bucket: fileMeta.GetBucket(),
	}

	stream, err := dr.client.Download(c, downloadRequest)
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
