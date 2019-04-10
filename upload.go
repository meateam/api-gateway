package main

import (
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
		ur.uploadInit(c)
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
	multipartForm, err := c.MultipartForm()
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed parsing multipart form data: %v", err))
		return
	}

	fileReader, header, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed getting file: %v", err))
		return
	}

	defer multipartForm.RemoveAll()

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

	reqUser := ur.extractRequestUser(c)
	if reqUser == nil {
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

	resp, err := ur.client.UploadMedia(c, ureq)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, resp.GetLocation())
	return
}

func (ur *uploadRouter) uploadInit(c *gin.Context) {
	reqUser := ur.extractRequestUser(c)

	uploadInitReq := &pb.UploadInitRequest{
		Key:    uuid.NewV4().String(),
		Bucket: reqUser.id,
	}

	contentType := c.PostForm("mimeType")
	if contentType != "" {
		uploadInitReq.ContentType = contentType
	}

	resp, err := ur.client.UploadInit(c, uploadInitReq)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, resp.GetUploadId())
	return
}

func (ur *uploadRouter) extractRequestUser(c *gin.Context) *user {
	contextUser, exists := c.Get("User")
	if exists != true {
		return nil
	}

	var reqUser user
	switch v := contextUser.(type) {
	case user:
		reqUser = v
		break
	default:
		return nil
	}

	return &reqUser
}
