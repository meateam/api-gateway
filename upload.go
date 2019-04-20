package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	fpb "github.com/meateam/file-service/protos"
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

type uploadRouter struct {
	client           pb.UploadClient
	fileClient       fpb.FileServiceClient
	uploadServiceURL string
}

func (ur *uploadRouter) setup(r *gin.Engine, fileConn *grpc.ClientConn) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(ur.uploadServiceURL, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	ur.client = pb.NewUploadClient(conn)

	if fileConn == nil {
		return nil, fmt.Errorf("file service connection is nil")
	}
	ur.fileClient = fpb.NewFileServiceClient(fileConn)

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

	upload, err := ur.fileClient.GetUploadByID(c, &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	uploadCompleteRequest := &pb.UploadCompleteRequest{
		UploadId: uploadID,
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
	}

	resp, err := ur.client.UploadComplete(c, uploadCompleteRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	fileName := uuid.NewV4().String()
	contentDisposition := c.GetHeader("Content-Disposition")
	if contentDisposition != "" {
		_, err := fmt.Sscanf(contentDisposition, "filename=%s", &fileName)
		if err != nil {
			fileName = uuid.NewV4().String()
		}
	}

	createFileResp, err := ur.fileClient.CreateFile(c, &fpb.CreateFileRequest{
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
		OwnerID:  reqUser.id,
		Size:     resp.GetContentLength(),
		Type:     resp.GetContentType(),
		FullName: fileName,
	})

	c.String(http.StatusCreated, createFileResp.GetId())
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
	fileName := ""
	contentDisposition := c.GetHeader("Content-Disposition")
	if contentDisposition != "" {
		_, err := fmt.Sscanf(contentDisposition, "filename=%s", &fileName)
		if err != nil {
			fileName = ""
		}
	}

	ur.uploadFile(c, fileReader, contentType, fileName)
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

	ur.uploadFile(c, file, contentType, fileHeader.Filename)
	return
}

func (ur *uploadRouter) uploadFile(c *gin.Context, fileReader io.ReadCloser, contentType string, filename string) {
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

	keyResp, err := ur.fileClient.GenerateKey(c, &fpb.GenerateKeyRequest{})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	key := keyResp.GetKey()
	fileFullName := uuid.NewV4().String()
	if filename != "" {
		fileFullName = filename
	}

	createFileResp, err := ur.fileClient.CreateFile(c, &fpb.CreateFileRequest{
		Key:      key,
		Bucket:   reqUser.id,
		OwnerID:  reqUser.id,
		Size:     int64(len(file)),
		Type:     contentType,
		FullName: fileFullName,
	})

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ureq := &pb.UploadMediaRequest{
		Key:    key,
		Bucket: reqUser.id,
		File:   file,
	}

	if contentType != "" {
		ureq.ContentType = contentType
	}

	_, err = ur.client.UploadMedia(c, ureq)
	if err != nil {
		ur.fileClient.DeleteFile(c, &fpb.DeleteFileRequest{
			Id: createFileResp.GetId(),
		})
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, createFileResp.GetId())
	return
}

func (ur *uploadRouter) uploadInit(c *gin.Context) {
	reqUser := extractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	createUploadResponse, err := ur.fileClient.CreateUpload(c, &fpb.CreateUploadRequest{
		Bucket: reqUser.id,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	uploadInitReq := &pb.UploadInitRequest{
		Key:    createUploadResponse.GetKey(),
		Bucket: reqUser.id,
	}

	contentType := c.GetHeader("Content-Type")
	if contentType != "" {
		uploadInitReq.ContentType = contentType
	}

	resp, err := ur.client.UploadInit(c, uploadInitReq)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	_, err = ur.fileClient.UpdateUploadID(c, &fpb.UpdateUploadIDRequest{
		Key:      createUploadResponse.GetKey(),
		Bucket:   reqUser.id,
		UploadID: resp.GetUploadId(),
	})

	// TODO: Handler update error, consider abstracting s3 upload id from client
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

	upload, err := ur.fileClient.GetUploadByID(c, &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

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
		Key:        upload.GetKey(),
		Bucket:     upload.GetBucket(),
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
