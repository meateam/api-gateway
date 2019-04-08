package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/upload-service/proto"
	"google.golang.org/grpc"
)

type uploadRouter struct {
	client           pb.UploadClient
	uploadServiceURL string
}

func (ur *uploadRouter) setup(r *gin.Engine) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(ur.uploadServiceURL, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	ur.client = pb.NewUploadClient(conn)
	r.POST("/upload", ur.upload)

	return conn, nil
}

func (ur *uploadRouter) upload(c *gin.Context) {
	ureq := &pb.UploadMediaRequest{
		Key:    "test.txt",
		Bucket: "testbucket",
		File:   []byte("Hello, World!"),
	}

	resp, err := ur.client.UploadMedia(context.Background(), ureq)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, resp.GetLocation())
}
