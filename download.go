package main

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/download-service/proto"
	"google.golang.org/grpc"
)

// Tfirot
const (
	downloadRequestFileKey = "masheukavua"
)

type downloadRouter struct {
	client             pb.DownloadClient
	downloadServiceURL string
}

func (dr *downloadRouter) setup(r *gin.Engine) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(dr.downloadServiceURL, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10 << 20)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	dr.client = pb.NewDownloadClient(conn)

	r.GET("/download/:id", dr.download)

	return conn, nil
}

func (dr *downloadRouter) download(c *gin.Context) {
	// TODO: Implement with file-service
	_ = c.Param("id")
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	filename := "shemkavua"
	contentType := "application/x-cd-image"
	// contentLength := fmt.Sprintf("%d", fileSize)

	downloadRequest := &pb.DownloadRequest{
		Key:    downloadRequestFileKey,
		Bucket: reqUser.id,
	}

	stream, err := dr.client.Download(c, downloadRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	// c.Header("Content-Length", contentLength)

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
