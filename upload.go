package main

import (
	"fmt"
	"io"
	"strings"
	"io/ioutil"
	"net/http"

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
	// fileName := upload.filename

	createFileResp, err := ur.fileClient.CreateFile(c, &fpb.CreateFileRequest{
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
		OwnerID:  reqUser.id,
		Size:     resp.GetContentLength(),
		Type:     resp.GetContentType(),
		FullName: fileName,
	})

	c.String(http.StatusOK, createFileResp.GetId())
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

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Header("x-uploadid", resp.GetUploadId())
	c.Status(http.StatusOK)
	return
}

func (ur *uploadRouter) uploadPart(c *gin.Context) {
	multipartReader, err := c.Request.MultipartReader()
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed reading multipart form data: %v", err))
		return
	}

	file, err := multipartReader.NextPart()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	uploadID, exists := c.GetQuery("uploadId")
	if exists != true {
		c.String(http.StatusBadRequest, "upload id is required")
		return
	}

	upload, err := ur.fileClient.GetUploadByID(c, &fpb.GetUploadByIDRequest{UploadID: uploadID})
	
	if err != nil {
		if strings.Contains(err.Error(), "Upload not found") {
			c.String(http.StatusBadRequest, "upload not found")
		} else {
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		
		return
	}
	
	fileRange := c.GetHeader("Content-Range")
	if fileRange == "" {
		c.String(http.StatusBadRequest, "Content-Range is required")
		return
	}

	rangeStart := int64(0)
	rangeEnd := int64(0)
	fileSize := int64(0)
	_, err = fmt.Sscanf(fileRange, "bytes %d-%d/%d", &rangeStart, &rangeEnd, &fileSize)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Content-Range is invalid: %v", err))
		return
	}

	bufSize := fileSize / 50
	if bufSize < 5 << 20 {
		bufSize = 5 << 20
	}

	if bufSize > 5120 << 20 {
		bufSize = 5120 << 20
	}

	partNumber := int64(1)
	stream, err := ur.client.UploadPart(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer stream.CloseSend()
	
	for {
		if rangeEnd - rangeStart + 1 < bufSize {
			bufSize = rangeEnd - rangeStart + 1
		}

		if bufSize == 0 {
			if rangeStart == fileSize {
				ur.uploadComplete(c)
				break
			}

			c.Status(http.StatusOK)
			break
		}
		
		buf := make([]byte, bufSize)
		bytesRead, err := io.ReadFull(file, buf)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			break
		}

		partRequest := &pb.UploadPartRequest{
			Part: buf,
			Key: upload.GetKey(),
			Bucket: upload.GetBucket(),
			PartNumber: partNumber,
			UploadId: uploadID,
		}

		err = stream.Send(partRequest)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			break
		}
		
		rangeStart += int64(bytesRead)
		partNumber++
	}

	return
}