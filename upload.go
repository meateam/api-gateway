package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/upload-service/proto"
	uuid "github.com/satori/go.uuid"
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
	uploadType, exists := c.GetQuery("uploadType")
	if exists != true {
		c.String(http.StatusBadRequest, "must specify uploadType")
		return
	}

	switch uploadType {
	case "media":
		ur.uploadMedia(c)
	default:
		c.String(http.StatusBadRequest, fmt.Sprintf("unknown uploadType=%v", uploadType))
		return
	}
}

func (ur *uploadRouter) uploadMedia(c *gin.Context) {
	fileReader := c.Request.Body
	if fileReader == nil {
		c.String(http.StatusBadRequest, "missing file body")
		return
	}

	if c.Request.ContentLength > 5<<20 { // 5MB
		c.String(http.StatusBadRequest, "file max size is 5MB")
		return
	}

	file, err := ioutil.ReadAll(fileReader)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	contextUser, exists := c.Get("User")
	if exists != true {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var reqUser user
	switch v := contextUser.(type) {
	case user:
		reqUser = v
		break
	default:
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ureq := &pb.UploadMediaRequest{
		Key:    uuid.NewV4().String(),
		Bucket: reqUser.id,
		File:   file,
	}

	contentType := c.GetHeader("Content-Type")
	if contentType != "" {
		ureq.ContentType = contentType
	}

	resp, err := ur.client.UploadMedia(context.Background(), ureq)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, resp.GetLocation())
	return
}
