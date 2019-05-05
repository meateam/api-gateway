package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	fpb "github.com/meateam/file-service/protos"
	pb "github.com/meateam/upload-service/proto"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	maxSimpleUploadSize = 5 << 20    // 5MB
	minPartUploadSize   = 5 << 20    // 5MB S3 limit
	maxPartUploadSize   = 5120 << 20 // 5GB S3 limit
	mediaUploadType     = "media"
	multipartUploadType = "multipart"
	resumableUploadType = "resumable"
)

type uploadRouter struct {
	uploadClient pb.UploadClient
	fileClient   fpb.FileServiceClient
}

type uploadInitBody struct {
	Title    string `json:"title"`
	MimeType string `json:"mimeType"`
}

func (ur *uploadRouter) setup(r *gin.Engine, uploadConn *grpc.ClientConn, fileConn *grpc.ClientConn) {
	ur.uploadClient = pb.NewUploadClient(uploadConn)
	ur.fileClient = fpb.NewFileServiceClient(fileConn)

	r.POST("/upload", ur.upload)

	return
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
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	uploadCompleteRequest := &pb.UploadCompleteRequest{
		UploadId: uploadID,
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
	}

	resp, err := ur.uploadClient.UploadComplete(c, uploadCompleteRequest)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	_, err = ur.fileClient.DeleteUploadByID(c, deleteUploadRequest)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	fileName := upload.Name

	createFileResp, err := ur.fileClient.CreateFile(c, &fpb.CreateFileRequest{
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
		OwnerID:  reqUser.id,
		Size:     resp.GetContentLength(),
		Type:     resp.GetContentType(),
		FullName: fileName,
	})

	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

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
		c.AbortWithError(int(status.Code(err)), err)
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
		c.AbortWithError(int(status.Code(err)), err)
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

	_, err = ur.uploadClient.UploadMedia(c, ureq)
	if err != nil {
		ur.fileClient.DeleteFile(c, &fpb.DeleteFileRequest{
			Id: createFileResp.GetId(),
		})

		c.AbortWithError(int(status.Code(err)), err)
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

	var reqBody uploadInitBody
	err := c.BindJSON(&reqBody)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid request body parameters")
		return
	}

	if reqBody.Title == "" {
		reqBody.Title = uuid.NewV4().String()
	}

	createUploadResponse, err := ur.fileClient.CreateUpload(c, &fpb.CreateUploadRequest{
		Bucket: reqUser.id,
		Name:   reqBody.Title,
	})

	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	uploadInitReq := &pb.UploadInitRequest{
		Key:    createUploadResponse.GetKey(),
		Bucket: reqUser.id,
	}

	uploadInitReq.ContentType = reqBody.MimeType
	if reqBody.MimeType == "" {
		uploadInitReq.ContentType = "application/octet-stream"
	}

	resp, err := ur.uploadClient.UploadInit(c, uploadInitReq)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	_, err = ur.fileClient.UpdateUploadID(c, &fpb.UpdateUploadIDRequest{
		Key:      createUploadResponse.GetKey(),
		Bucket:   reqUser.id,
		UploadID: resp.GetUploadId(),
	})

	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
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
		c.AbortWithError(int(status.Code(err)), err)
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
	if bufSize < minPartUploadSize {
		bufSize = minPartUploadSize
	}

	if bufSize > maxPartUploadSize {
		bufSize = maxPartUploadSize
	}

	partNumber := int64(1)
	stream, err := ur.uploadClient.UploadPart(c)
	if err != nil {
		c.AbortWithError(int(status.Code(err)), err)
		return
	}

	errc := make(chan error, 1)
	defer close(errc)
	responseWG := sync.WaitGroup{}
	responseWG.Add(1)
	go func() {
		defer responseWG.Done()
		for {
			partResponse, err := stream.Recv()

			// Upload response that all parts have finished uploading.
			if err == io.EOF {
				ur.uploadComplete(c)
				return
			}

			// If there's an error uploading any part then abort the upload process,
			// and delete the parts that have finished uploading.
			if err != nil || partResponse.GetCode() == 500 {
				if err != nil {
					errc <- err
				}

				if partResponse.GetCode() == 500 {
					errc <- fmt.Errorf(partResponse.GetMessage())
				}

				abortUploadRequest := &pb.UploadAbortRequest{
					UploadId: upload.GetUploadID(),
					Key:      upload.GetKey(),
					Bucket:   upload.GetBucket(),
				}

				ur.uploadClient.UploadAbort(c, abortUploadRequest)

				deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
					UploadID: upload.GetUploadID(),
				}

				ur.fileClient.DeleteUploadByID(c, deleteUploadRequest)
				return
			}
		}
	}()

	for {
		// If there's an error stop uploading file parts.
		// Otherwise continue uploading the remaining parts.
		select {
		case err := <-errc:
			c.AbortWithError(int(status.Code(err)), err)
			break
		default:
		}

		if rangeEnd-rangeStart+1 < bufSize {
			bufSize = rangeEnd - rangeStart + 1
		}

		if bufSize == 0 {
			break
		}

		buf := make([]byte, bufSize)
		bytesRead, err := io.ReadFull(file, buf)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				abortUploadRequest := &pb.UploadAbortRequest{
					UploadId: upload.GetUploadID(),
					Key:      upload.GetKey(),
					Bucket:   upload.GetBucket(),
				}

				ur.uploadClient.UploadAbort(c, abortUploadRequest)

				deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
					UploadID: upload.GetUploadID(),
				}

				ur.fileClient.DeleteUploadByID(c, deleteUploadRequest)
				c.Abort()
				break
			}

			c.AbortWithError(http.StatusInternalServerError, err)
			break
		}

		partRequest := &pb.UploadPartRequest{
			Part:       buf,
			Key:        upload.GetKey(),
			Bucket:     upload.GetBucket(),
			PartNumber: partNumber,
			UploadId:   uploadID,
		}

		err = stream.Send(partRequest)
		if err != nil {
			c.AbortWithError(int(status.Code(err)), err)
			return
		}

		rangeStart += int64(bytesRead)
		partNumber++
	}

	// Close the stream after finishing uploading all file parts.
	stream.CloseSend()
	responseWG.Wait()

	return
}
