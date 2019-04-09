package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/upload-service/proto"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc"
)

const (
	maxSimpleUploadSize = 5 << 20 // 5MB
	mediaUploadType     = "media"
	multipartUploadType = "multipart"
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
	case mediaUploadType:
		ur.uploadMedia(c)
		break
	case multipartUploadType:
		ur.uploadMultipart(c)
		break
	default:
		c.String(http.StatusBadRequest, fmt.Sprintf("unknown uploadType=%v", uploadType))
		return
	}
	return
}

func (ur *uploadRouter) uploadMedia(c *gin.Context) {
	fileReader := c.Request.Body
	if fileReader == nil {
		c.String(http.StatusBadRequest, "missing file body")
		return
	}

	if c.Request.ContentLength > maxSimpleUploadSize {
		c.String(http.StatusBadRequest, fmt.Sprintf("max file size exceeded %d", maxSimpleUploadSize))
		return
	}

	contentType := c.GetHeader("Content-Type")

	ur.uploadFile(c, fileReader, contentType)
	return
}

func (ur *uploadRouter) uploadMultipart(c *gin.Context) {
	fileReader, header, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed getting file: %v", err))
	}

	defer c.Request.MultipartForm.RemoveAll()

	if header.Size > maxSimpleUploadSize {
		c.String(http.StatusBadRequest, fmt.Sprintf("max file size exceeded %d", maxSimpleUploadSize))
		return
	}

	contentType := header.Header.Get("Content-Type")

	ur.uploadFile(c, fileReader, contentType)
	return
}

func (ur *uploadRouter) uploadFile(c *gin.Context, fileReader io.ReadCloser, contentType string) {
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
