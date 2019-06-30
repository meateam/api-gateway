package upload

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	fpb "github.com/meateam/file-service/proto"
	upb "github.com/meateam/upload-service/proto"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	// MaxSimpleUploadSize 5MB.
	MaxSimpleUploadSize = 5 << 20

	// MinPartUploadSize 5MB S3 min limit.
	MinPartUploadSize = 5 << 20

	// MaxPartUploadSize 5GB S3 max limit.
	MaxPartUploadSize = 5120 << 20

	// MediaUploadType media upload type name.
	MediaUploadType = "media"

	// MultipartUploadType multipart upload type name.
	MultipartUploadType = "multipart"

	// ResumableUploadType resumable upload type name.
	ResumableUploadType = "resumable"

	// ParentQueryStringKey parent query string key name.
	ParentQueryStringKey = "parent"

	// CustomContentLength custom content length header name.
	CustomContentLength = "X-Content-Length"
)

// Router is a structure that handles upload requests.
type Router struct {
	uploadClient upb.UploadClient
	fileClient   fpb.FileServiceClient
	logger       *logrus.Logger
	mu           sync.Mutex
}

type uploadInitBody struct {
	Title    string `json:"title"`
	MimeType string `json:"mimeType"`
}

// NewRouter creates a new Router, and initializes clients of Upload Service
// and File Service with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(uploadConn *grpc.ClientConn, fileConn *grpc.ClientConn, logger *logrus.Logger) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.uploadClient = upb.NewUploadClient(uploadConn)
	r.fileClient = fpb.NewFileServiceClient(fileConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.POST("/upload", r.upload)

	return
}

func (r *Router) upload(c *gin.Context) {
	uploadType, exists := c.GetQuery("uploadType")
	if exists != true {
		r.uploadInit(c)
		return
	}

	switch uploadType {
	case MediaUploadType:
		r.uploadMedia(c)
		break
	case MultipartUploadType:
		r.uploadMultipart(c)
		break
	case ResumableUploadType:
		r.uploadPart(c)
		break
	default:
		c.String(http.StatusBadRequest, fmt.Sprintf("unknown uploadType=%v", uploadType))
		return
	}
	return
}

func (r *Router) uploadComplete(c *gin.Context) {
	uploadID, exists := c.GetQuery("uploadId")
	if exists != true {
		c.String(http.StatusBadRequest, "upload id is required")
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	upload, err := r.fileClient.GetUploadByID(c.Request.Context(), &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	uploadCompleteRequest := &upb.UploadCompleteRequest{
		UploadId: uploadID,
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
	}

	resp, err := r.uploadClient.UploadComplete(c.Request.Context(), uploadCompleteRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	_, err = r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	createFileResp, err := r.fileClient.CreateFile(c.Request.Context(), &fpb.CreateFileRequest{
		Key:     upload.GetKey(),
		Bucket:  upload.GetBucket(),
		OwnerID: reqUser.ID,
		Size:    resp.GetContentLength(),
		Type:    resp.GetContentType(),
		Name:    upload.Name,
		Parent:  c.Query(ParentQueryStringKey),
	})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	c.String(http.StatusOK, createFileResp.GetId())
	return
}

func (r *Router) uploadMedia(c *gin.Context) {
	fileReader := c.Request.Body
	if fileReader == nil {
		c.String(http.StatusBadRequest, "missing file body")
		return
	}

	if c.Request.ContentLength > MaxSimpleUploadSize {
		c.String(http.StatusBadRequest, fmt.Sprintf("max file size exceeded %d", MaxSimpleUploadSize))
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

	r.uploadFile(c, fileReader, contentType, fileName)
	return
}

func (r *Router) uploadMultipart(c *gin.Context) {
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

	if fileHeader.Size > MaxSimpleUploadSize {
		c.String(http.StatusBadRequest, fmt.Sprintf("max file size exceeded %d", MaxSimpleUploadSize))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")

	r.uploadFile(c, file, contentType, fileHeader.Filename)
	return
}

func (r *Router) uploadFile(c *gin.Context, fileReader io.ReadCloser, contentType string, filename string) {
	file, err := ioutil.ReadAll(fileReader)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	keyResp, err := r.fileClient.GenerateKey(c.Request.Context(), &fpb.GenerateKeyRequest{})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	key := keyResp.GetKey()
	fileFullName := uuid.NewV4().String()
	if filename != "" {
		fileFullName = filename
	}

	createFileResp, err := r.fileClient.CreateFile(c.Request.Context(), &fpb.CreateFileRequest{
		Key:     key,
		Bucket:  reqUser.ID,
		OwnerID: reqUser.ID,
		Size:    int64(len(file)),
		Type:    contentType,
		Name:    fileFullName,
		Parent:  c.Query(ParentQueryStringKey),
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	ureq := &upb.UploadMediaRequest{
		Key:    key,
		Bucket: reqUser.ID,
		File:   file,
	}

	if contentType != "" {
		ureq.ContentType = contentType
	}

	_, err = r.uploadClient.UploadMedia(c.Request.Context(), ureq)
	if err != nil {
		r.fileClient.DeleteFile(c.Request.Context(), &fpb.DeleteFileRequest{
			Id: createFileResp.GetId(),
		})

		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	c.String(http.StatusOK, createFileResp.GetId())
	return
}

func (r *Router) uploadInit(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var reqBody uploadInitBody
	if err := c.BindJSON(&reqBody); err != nil {
		c.String(http.StatusBadRequest, "invalid request body parameters")
		return
	}

	if reqBody.Title == "" {
		reqBody.Title = uuid.NewV4().String()
	}

	fileSize, err := strconv.ParseInt(c.Request.Header.Get(CustomContentLength), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Content-Length is invalid")
		return
	}

	if fileSize < 0 {
		fileSize = 0
	}

	createUploadResponse, err := r.fileClient.CreateUpload(c.Request.Context(), &fpb.CreateUploadRequest{
		Bucket:  reqUser.ID,
		Name:    reqBody.Title,
		OwnerID: reqUser.ID,
		Parent:  c.Query(ParentQueryStringKey),
		Size:    fileSize,
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	uploadInitReq := &upb.UploadInitRequest{
		Key:    createUploadResponse.GetKey(),
		Bucket: reqUser.ID,
	}

	uploadInitReq.ContentType = reqBody.MimeType
	if reqBody.MimeType == "" {
		uploadInitReq.ContentType = "application/octet-stream"
	}

	resp, err := r.uploadClient.UploadInit(c.Request.Context(), uploadInitReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	_, err = r.fileClient.UpdateUploadID(c.Request.Context(), &fpb.UpdateUploadIDRequest{
		Key:      createUploadResponse.GetKey(),
		Bucket:   reqUser.ID,
		UploadID: resp.GetUploadId(),
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

		return
	}

	c.Header("x-uploadid", resp.GetUploadId())
	c.Status(http.StatusOK)
	return
}

func (r *Router) uploadPart(c *gin.Context) {
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

	upload, err := r.fileClient.GetUploadByID(c.Request.Context(), &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
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
	if bufSize < MinPartUploadSize {
		bufSize = MinPartUploadSize
	}

	if bufSize > MaxPartUploadSize {
		bufSize = MaxPartUploadSize
	}

	partNumber := int64(1)

	span, spanCtx := loggermiddleware.StartSpan(c.Request.Context(), "/upload.Upload/UploadPart")
	defer span.End()

	stream, err := r.uploadClient.UploadPart(spanCtx)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			r.logger.Errorf("%v", err)
		}

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
				r.uploadComplete(c)
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

				abortUploadRequest := &upb.UploadAbortRequest{
					UploadId: upload.GetUploadID(),
					Key:      upload.GetKey(),
					Bucket:   upload.GetBucket(),
				}

				r.uploadClient.UploadAbort(spanCtx, abortUploadRequest)

				deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
					UploadID: upload.GetUploadID(),
				}

				r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
				return
			}
		}
	}()

	for {
		// If there's an error stop uploading file parts.
		// Otherwise continue uploading the remaining parts.
		select {
		case err := <-errc:
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			if err := c.AbortWithError(httpStatusCode, err); err != nil {
				r.logger.Errorf("%v", err)
			}

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
				abortUploadRequest := &upb.UploadAbortRequest{
					UploadId: upload.GetUploadID(),
					Key:      upload.GetKey(),
					Bucket:   upload.GetBucket(),
				}

				r.uploadClient.UploadAbort(spanCtx, abortUploadRequest)

				deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
					UploadID: upload.GetUploadID(),
				}

				r.fileClient.DeleteUploadByID(c.Request.Context(), deleteUploadRequest)
				c.Abort()
				break
			}

			c.AbortWithError(http.StatusInternalServerError, err)
			break
		}

		partRequest := &upb.UploadPartRequest{
			Part:       buf,
			Key:        upload.GetKey(),
			Bucket:     upload.GetBucket(),
			PartNumber: partNumber,
			UploadId:   uploadID,
		}

		if err := stream.Send(partRequest); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			if abortErr := c.AbortWithError(httpStatusCode, err); abortErr != nil {
				r.logger.Errorf("%v", abortErr)
			}

			if err == io.EOF {
				responseWG.Wait()
				c.Request.Body.Close()
				return
			}

			r.logger.Errorf("%v", err)
			break
		}

		rangeStart += int64(bytesRead)
		partNumber++
	}

	// Close the stream after finishing uploading all file parts.
	stream.CloseSend()
	responseWG.Wait()

	return
}
