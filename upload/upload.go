package upload

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
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

	// ParentQueryKey parent query string key name.
	ParentQueryKey = "parent"

	// ContentLengthCustomHeader content length custom header name.
	ContentLengthCustomHeader = "X-Content-Length"

	// ContentRangeHeader content-range header name.
	ContentRangeHeader = "Content-Range"

	// UploadIDQueryKey the upload id query string key name.
	UploadIDQueryKey = "uploadId"

	// UploadIDCustomHeader upload id custom header name.
	UploadIDCustomHeader = "x-uploadid"

	// DefaultContentLength the default content length of a file.
	DefaultContentLength = "application/octet-stream"

	// ContentTypeHeader content type header name.
	ContentTypeHeader = "Content-Type"

	// FileFormName the key of the file in a form.
	FileFormName = "file"

	// ContentDispositionHeader content-disposition header name.
	ContentDispositionHeader = "Content-Disposition"

	// UploadTypeQueryKey the upload type query string key name.
	UploadTypeQueryKey = "uploadType"
)

// Router is a structure that handles upload requests.
type Router struct {
	uploadClient upb.UploadClient
	fileClient   fpb.FileServiceClient
	logger       *logrus.Logger
	mu           sync.Mutex
}

// uploadInitBody is a structure of the json body of upload init request.
type uploadInitBody struct {
	Title    string `json:"title"`
	MimeType string `json:"mimeType"`
}

// resumableFileUploadProgress is a structure of a resumable file upload progress.
type resumableFileUploadProgress struct {
	rangeStart int64
	rangeEnd   int64
	bufSize    int64
	upload     *fpb.GetUploadByIDResponse
	file       *multipart.Part
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
	rg.POST("/upload", r.Upload)
}

// Upload is the request handler for /upload request.
func (r *Router) Upload(c *gin.Context) {
	uploadType, exists := c.GetQuery(UploadTypeQueryKey)
	if !exists {
		r.UploadInit(c)
		return
	}

	switch uploadType {
	case MediaUploadType:
		r.UploadMedia(c)
	case MultipartUploadType:
		r.UploadMultipart(c)
	case ResumableUploadType:
		r.UploadPart(c)
	default:
		c.String(http.StatusBadRequest, fmt.Sprintf("unknown uploadType=%v", uploadType))
		return
	}
}

// UploadComplete completes a resumable file upload and creates the uploaded file.
func (r *Router) UploadComplete(c *gin.Context) {
	uploadID, exists := c.GetQuery(UploadIDQueryKey)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", UploadIDQueryKey))
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
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

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
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

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
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	createFileResp, err := r.fileClient.CreateFile(c.Request.Context(), &fpb.CreateFileRequest{
		Key:     upload.GetKey(),
		Bucket:  upload.GetBucket(),
		OwnerID: reqUser.ID,
		Size:    resp.GetContentLength(),
		Type:    resp.GetContentType(),
		Name:    upload.Name,
		Parent:  c.Query(ParentQueryKey),
	})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.String(http.StatusOK, createFileResp.GetId())
}

// UploadMedia uploads a file from request's body.
func (r *Router) UploadMedia(c *gin.Context) {
	fileReader := c.Request.Body
	if fileReader == nil {
		c.String(http.StatusBadRequest, "missing file body")
		return
	}

	if c.Request.ContentLength > MaxSimpleUploadSize {
		c.String(http.StatusBadRequest, fmt.Sprintf("max file size exceeded %d", MaxSimpleUploadSize))
		return
	}

	contentType := c.GetHeader(ContentTypeHeader)
	fileName := ""
	contentDisposition := c.GetHeader(ContentDispositionHeader)
	if contentDisposition != "" {
		_, err := fmt.Sscanf(contentDisposition, "filename=%s", &fileName)
		if err != nil {
			fileName = ""
		}
	}

	r.UploadFile(c, fileReader, contentType, fileName)
}

// UploadMultipart uploads a file from multipart/form-data request.
func (r *Router) UploadMultipart(c *gin.Context) {
	multipartForm, err := c.MultipartForm()
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed parsing multipart form data: %v", err))
		return
	}
	defer loggermiddleware.LogError(r.logger, multipartForm.RemoveAll())

	fileHeader, err := c.FormFile(FileFormName)
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
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
		return
	}

	contentType := fileHeader.Header.Get(ContentTypeHeader)

	r.UploadFile(c, file, contentType, fileHeader.Filename)
}

// UploadFile uploads file from fileReader of type contentType with name filename to
// upload service and creates it in file service.
func (r *Router) UploadFile(c *gin.Context, fileReader io.ReadCloser, contentType string, filename string) {
	file, err := ioutil.ReadAll(fileReader)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
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
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

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
		Parent:  c.Query(ParentQueryKey),
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

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
		if _, err := r.fileClient.DeleteFile(c.Request.Context(), &fpb.DeleteFileRequest{
			Id: createFileResp.GetId(),
		}); err != nil {
			r.logger.Errorf("%v", err)
		}

		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.String(http.StatusOK, createFileResp.GetId())
}

// UploadInit initiates a resumable upload to upload a large file to.
func (r *Router) UploadInit(c *gin.Context) {
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

	fileSize, err := strconv.ParseInt(c.Request.Header.Get(ContentLengthCustomHeader), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is invalid", ContentLengthCustomHeader))
		return
	}

	if fileSize < 0 {
		fileSize = 0
	}

	createUploadResponse, err := r.fileClient.CreateUpload(c.Request.Context(), &fpb.CreateUploadRequest{
		Bucket:  reqUser.ID,
		Name:    reqBody.Title,
		OwnerID: reqUser.ID,
		Parent:  c.Query(ParentQueryKey),
		Size:    fileSize,
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	uploadInitReq := &upb.UploadInitRequest{
		Key:    createUploadResponse.GetKey(),
		Bucket: reqUser.ID,
	}

	uploadInitReq.ContentType = reqBody.MimeType
	if reqBody.MimeType == "" {
		uploadInitReq.ContentType = DefaultContentLength
	}

	resp, err := r.uploadClient.UploadInit(c.Request.Context(), uploadInitReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	_, err = r.fileClient.UpdateUploadID(c.Request.Context(), &fpb.UpdateUploadIDRequest{
		Key:      createUploadResponse.GetKey(),
		Bucket:   reqUser.ID,
		UploadID: resp.GetUploadId(),
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.Header(UploadIDCustomHeader, resp.GetUploadId())
	c.Status(http.StatusOK)
}

// UploadPart uploads a multipart file to a resumable upload.
func (r *Router) UploadPart(c *gin.Context) {
	multipartReader, err := c.Request.MultipartReader()
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("failed reading multipart form data: %v", err))
		return
	}

	file, err := multipartReader.NextPart()
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
		return
	}
	defer file.Close()

	uploadID, exists := c.GetQuery(UploadIDQueryKey)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", UploadIDQueryKey))
		return
	}

	upload, err := r.fileClient.GetUploadByID(c.Request.Context(), &fpb.GetUploadByIDRequest{UploadID: uploadID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	fileRange := c.GetHeader(ContentRangeHeader)
	if fileRange == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", ContentRangeHeader))
		return
	}

	rangeStart := int64(0)
	rangeEnd := int64(0)
	fileSize := int64(0)
	_, err = fmt.Sscanf(fileRange, "bytes %d-%d/%d", &rangeStart, &rangeEnd, &fileSize)
	if err != nil {
		contentRangeErr := fmt.Errorf("%s is invalid: %v", ContentRangeHeader, err)
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, contentRangeErr))

		return
	}

	bufSize := r.calculateBufSize(fileSize)

	span, spanCtx := loggermiddleware.StartSpan(c.Request.Context(), "/upload.Upload/UploadPart")
	defer span.End()

	stream, err := r.uploadClient.UploadPart(spanCtx)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	errc := make(chan error, 1)
	defer close(errc)

	responseWG := sync.WaitGroup{}
	responseWG.Add(1)

	go r.HandleError(spanCtx, c, errc, &responseWG, stream, upload)

	progress := &resumableFileUploadProgress{
		rangeStart: rangeStart,
		rangeEnd:   rangeEnd,
		bufSize:    bufSize,
		file:       file,
		upload:     upload,
	}

	if err := r.HandleUpload(
		spanCtx,
		c,
		errc,
		&responseWG,
		progress,
		stream); err == io.EOF {
		return
	}

	// Close the stream after finishing uploading all file parts.
	loggermiddleware.LogError(r.logger, stream.CloseSend())
	responseWG.Wait()
}

// HandleError receive messages from bi-directional stream and handles upload service
// errors. If received non-nil and non-EOF errors it sends the error through errc,
// and aborts the upload.
func (r *Router) HandleError(
	ctx context.Context,
	c *gin.Context,
	errc chan error,
	wg *sync.WaitGroup,
	stream upb.Upload_UploadPartClient,
	upload *fpb.GetUploadByIDResponse) {
	defer wg.Done()
	for {
		partResponse, err := stream.Recv()

		// Upload response that all parts have finished uploading.
		if err == io.EOF {
			r.UploadComplete(c)
			return
		}

		// If there's an error uploading any part then abort the upload process.
		if err != nil || partResponse.GetCode() == 500 {
			if err != nil {
				errc <- err
			}

			if partResponse.GetCode() == 500 {
				errc <- fmt.Errorf(partResponse.GetMessage())
			}

			return
		}
	}
}

// HandleUpload sends to bi-directional stream file found in progress. Upload file bytes
// from progress.rangeStart to progress.rangeEnd sending in parts in size of
// progress.bufSize.
// Receives errors from errc, if any error is received then the operation would be aborted.
// Returns nil error when sending is done with no errors, if stream is broken
// then returns io.EOF.
func (r *Router) HandleUpload(
	ctx context.Context,
	c *gin.Context,
	errc chan error,
	wg *sync.WaitGroup,
	progress *resumableFileUploadProgress,
	stream upb.Upload_UploadPartClient) error {
	partNumber := int64(1)

	for {
		// If there's an error stop uploading file parts.
		// Otherwise continue uploading the remaining parts.
		select {
		case err := <-errc:
			loggermiddleware.LogError(r.logger, r.AbortUpload(ctx, progress.upload))

			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			break
		default:
		}

		if progress.rangeEnd-progress.rangeStart+1 < progress.bufSize {
			progress.bufSize = progress.rangeEnd - progress.rangeStart + 1
		}

		if progress.bufSize == 0 {
			break
		}

		buf := make([]byte, progress.bufSize)
		bytesRead, err := io.ReadFull(progress.file, buf)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				loggermiddleware.LogError(r.logger, r.AbortUpload(ctx, progress.upload))
				c.Abort()

				break
			}

			loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))

			break
		}

		partRequest := &upb.UploadPartRequest{
			Part:       buf,
			Key:        progress.upload.GetKey(),
			Bucket:     progress.upload.GetBucket(),
			PartNumber: partNumber,
			UploadId:   progress.upload.GetUploadID(),
		}

		if err := stream.Send(partRequest); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			// Stream is broken, wait for wg and return.
			if err == io.EOF {
				wg.Wait()
				c.Request.Body.Close()

				return io.EOF
			}

			break
		}

		progress.rangeStart += int64(bytesRead)
		partNumber++
	}

	return nil
}

// AbortUpload aborts upload in upload service and file service, returns non-nil error if any occurred.
func (r *Router) AbortUpload(ctx context.Context, upload *fpb.GetUploadByIDResponse) error {
	abortUploadRequest := &upb.UploadAbortRequest{
		UploadId: upload.GetUploadID(),
		Key:      upload.GetKey(),
		Bucket:   upload.GetBucket(),
	}

	if _, err := r.uploadClient.UploadAbort(ctx, abortUploadRequest); err != nil {
		return err
	}

	deleteUploadRequest := &fpb.DeleteUploadByIDRequest{
		UploadID: upload.GetUploadID(),
	}

	if _, err := r.fileClient.DeleteUploadByID(ctx, deleteUploadRequest); err != nil {
		return err
	}

	return nil
}

// calculateBufSize gets a file size and calculates the size of the buffer to read the file
// and stream it to upload service.
func (r *Router) calculateBufSize(fileSize int64) int64 {
	bufSize := fileSize / 50
	if bufSize < MinPartUploadSize {
		bufSize = MinPartUploadSize
	}

	if bufSize > MaxPartUploadSize {
		bufSize = MaxPartUploadSize
	}

	return bufSize
}
