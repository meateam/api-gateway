package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	pb "github.com/meateam/upload-service/proto"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc"
)

const (
	maxSimpleUploadSize = 5 << 20 // 5MB
	minPartUploadSize   = 5 << 20 // 5MB S3 limit
	mediaUploadType     = "media"
	multipartUploadType = "multipart"
	resumableUploadType = "resumable"
)

// Tfirot
const (
	resumableUploadKey = "masheukavua"
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
	r.PUT("/upload", ur.uploadComplete)

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
	case resumableUploadType:
		ur.uploadPart(c)
		break
	default:
		c.String(http.StatusBadRequest, fmt.Sprintf("unknown uploadType=%v", uploadType))
		return
	}
	return
}

func (ur *uploadRouter) uploadComplete(c *gin.Context) {
	uploadID, exists := c.GetQuery("uploadId")
	if exists != true {
		c.String(http.StatusBadRequest, "upload id is required")
		return
	}

	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	uploadCompleteRequest := &pb.UploadCompleteRequest{
		UploadId: uploadID,
		Key:      resumableUploadKey,
		Bucket:   reqUser.id,
	}

	resp, err := ur.client.UploadComplete(c, uploadCompleteRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusCreated, resp.GetLocation())
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
	defer multipartForm.RemoveAll()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed getting file: %v", err))
		return
	}

	if fileHeader.Size > maxSimpleUploadSize {
		c.String(http.StatusBadRequest, fmt.Sprintf("max file size exceeded %d", maxSimpleUploadSize))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")

	ur.uploadFile(c, file, contentType)
	return
}

func (ur *uploadRouter) uploadFile(c *gin.Context, fileReader io.ReadCloser, contentType string) {
	file, err := ioutil.ReadAll(fileReader)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	reqUser := extractRequestUser(c)
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
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	uploadInitReq := &pb.UploadInitRequest{
		Key:    resumableUploadKey,
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

func (ur *uploadRouter) uploadPart(c *gin.Context) {
	multipartForm, err := c.MultipartForm()
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed parsing multipart form data: %v", err))
		return
	}
	defer multipartForm.RemoveAll()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed getting file: %v", err))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	uploadID, exists := c.GetPostForm("uploadId")
	if exists != true {
		c.String(http.StatusBadRequest, "upload id is required")
		return
	}

	key := resumableUploadKey
	chunkIndex, exists := c.GetPostForm("chunkIndex")
	if exists != true {
		c.String(http.StatusBadRequest, "chunk index is required")
		return
	}

	partNumber, err := strconv.ParseInt(chunkIndex, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("chunk index is invalid: %v", err))
		return
	}

	partBytes, err := ioutil.ReadAll(file)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	uploadPartInput := &pb.UploadPartRequest{
		UploadId:   uploadID,
		Key:        key,
		Bucket:     reqUser.id,
		Part:       partBytes,
		PartNumber: partNumber,
	}

	_, err = ur.client.UploadPart(c, uploadPartInput)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
	return
}
