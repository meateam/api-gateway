package file

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/klauspost/compress/zip"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/download-service/download"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	upb "github.com/meateam/upload-service/proto"
	minioutil "github.com/minio/minio/pkg/ioutil"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileParent is a constant for file parent parameter in a request
	ParamFileParent = "parent"

	// ParamFileName is a constant for file name parameter in a request
	ParamFileName = "name"

	// ParamFileType is a constant for file type parameter in a request
	ParamFileType = "type"

	// ParamFileDescription is a constant for file description parameter in a request
	ParamFileDescription = "description"

	// ParamFileSize is a constant for file size parameter in a request
	ParamFileSize = "size"

	// ParamFileCreatedAt is a constant for file created at parameter in a request
	ParamFileCreatedAt = "createdAt"

	// ParamFileUpdatedAt is a constant for file updated at parameter in a request
	ParamFileUpdatedAt = "updatedAt"

	// FolderContentType is the custom content type of a folder.
	FolderContentType = "application/vnd.drive.folder"
)

var (
	// Some standard object extensions which we strictly dis-allow for compression.
	standardExcludeCompressExtensions = []string{".gz", ".bz2", ".rar", ".zip", ".7z", ".xz", ".mp4", ".mkv", ".mov"}

	// Some standard content-types which we strictly dis-allow for compression.
	standardExcludeCompressContentTypes = []string{"video/*", "audio/*", "application/zip", "application/x-gzip", "application/x-zip-compressed", " application/x-compress", "application/x-spoon"}
)

// Router is a structure that handles upload requests.
type Router struct {
	downloadClient dpb.DownloadClient
	fileClient     fpb.FileServiceClient
	uploadClient   upb.UploadClient
	logger         *logrus.Logger
}

// getFileByIDResponse is a structure used for parsing fpb.File to a json file metadata response.
type getFileByIDResponse struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Description string `json:"description,omitempty"`
	OwnerID     string `json:"ownerId,omitempty"`
	Parent      string `json:"parent,omitempty"`
	CreatedAt   int64  `json:"createdAt,omitempty"`
	UpdatedAt   int64  `json:"updatedAt,omitempty"`
}

type partialFile struct {
	ID          string  `json:"id,omitempty"`
	Name        string  `json:"name,omitempty"`
	Type        string  `json:"type,omitempty"`
	Size        int64   `json:"size,omitempty"`
	Description string  `json:"description,omitempty"`
	OwnerID     string  `json:"ownerId,omitempty"`
	Parent      *string `json:"parent,omitempty"`
	CreatedAt   int64   `json:"createdAt,omitempty"`
	UpdatedAt   int64   `json:"updatedAt,omitempty"`
}

type updateFilesRequest struct {
	IDList      []string    `json:"idList"`
	PartialFile partialFile `json:"partialFile"`
}

// NewRouter creates a new Router, and initializes clients of File Service
// and Download Service with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	fileConn *grpc.ClientConn,
	downloadConn *grpc.ClientConn,
	uploadConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.fileClient = fpb.NewFileServiceClient(fileConn)
	r.downloadClient = dpb.NewDownloadClient(downloadConn)
	r.uploadClient = upb.NewUploadClient(uploadConn)

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/files", r.GetFilesByFolder)
	rg.GET("/files/:id", r.GetFileByID)
	rg.DELETE("/files/:id", r.DeleteFileByID)
	rg.PUT("/files/:id", r.UpdateFile)
	rg.PUT("/files", r.UpdateFiles)
	rg.POST("/files/zip", r.DownloadZip)
}

// GetFileByID is the request handler for GET /files/:id
func (r *Router) GetFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	alt := c.Query("alt")
	if alt == "media" {
		r.Download(c)
		return
	}

	isUserAllowed := r.HandleUserFilePermission(c, fileID)
	if !isUserAllowed {
		return
	}

	getFileByIDRequest := &fpb.GetByFileByIDRequest{
		Id: fileID,
	}

	file, err := r.fileClient.GetFileByID(c.Request.Context(), getFileByIDRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	responseFile, err := createGetFileResponse(file)
	if err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, responseFile)
}

// Extracts parameters from request query to a map, non-existing parameter has a value of ""
func queryParamsToMap(c *gin.Context, paramNames ...string) map[string]string {
	paramMap := make(map[string]string)
	for _, paramName := range paramNames {
		param, exists := c.GetQuery(paramName)
		if exists {
			paramMap[paramName] = param
		} else {
			paramMap[paramName] = ""
		}
	}
	return paramMap
}

// Converts a string to int64, 0 is returned on failure
func stringToInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		n = 0
	}
	return n
}

// GetFilesByFolder is the request handler for GET /files request.
func (r *Router) GetFilesByFolder(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filesParent, exists := c.GetQuery(ParamFileParent)
	if exists {
		isUserAllowed := r.HandleUserFilePermission(c, filesParent)
		if !isUserAllowed {
			return
		}
	}

	paramMap := queryParamsToMap(c, ParamFileName, ParamFileType, ParamFileDescription, ParamFileSize,
		ParamFileCreatedAt, ParamFileUpdatedAt)

	fileFilter := fpb.File{
		Name:        paramMap[ParamFileName],
		Type:        paramMap[ParamFileType],
		Description: paramMap[ParamFileDescription],
		Size:        stringToInt64(paramMap[ParamFileSize]),
		CreatedAt:   stringToInt64(paramMap[ParamFileCreatedAt]),
		UpdatedAt:   stringToInt64(paramMap[ParamFileUpdatedAt]),
	}

	filesResp, err := r.fileClient.GetFilesByFolder(
		c.Request.Context(),
		&fpb.GetFilesByFolderRequest{OwnerID: reqUser.ID, FolderID: filesParent, QueryFile: &fileFilter},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	files := filesResp.GetFiles()
	responseFiles := make([]*getFileByIDResponse, 0, len(files))
	for _, file := range files {
		responseFile, err := createGetFileResponse(file)
		if err != nil {
			loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))
			return
		}

		responseFiles = append(responseFiles, responseFile)
	}

	c.JSON(http.StatusOK, responseFiles)
}

// DeleteFileByID is the request handler for DELETE /files/:id request.
func (r *Router) DeleteFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	isUserAllowed := r.HandleUserFilePermission(c, fileID)
	if !isUserAllowed {
		return
	}

	deleteFileRequest := &fpb.DeleteFileRequest{
		Id: fileID,
	}
	deleteFileResponse, err := r.fileClient.DeleteFile(c.Request.Context(), deleteFileRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	bucketKeysMap := make(map[string][]string)
	ids := make([]string, len(deleteFileResponse.GetFiles()))
	for i, file := range deleteFileResponse.GetFiles() {
		bucketKeysMap[file.GetBucket()] = append(bucketKeysMap[file.GetBucket()], file.GetKey())
		ids[i] = file.GetId()
	}

	var wg sync.WaitGroup

	for bucket, keys := range bucketKeysMap {
		wg.Add(1)
		go func(bucket string, keys []string) {
			DeleteObjectRequest := &upb.DeleteObjectsRequest{
				Bucket: bucket,
				Keys:   keys,
			}
			deleteObjectResponse, err := r.uploadClient.DeleteObjects(c.Request.Context(), DeleteObjectRequest)
			if err != nil || len(deleteObjectResponse.GetFailed()) > 0 {
				loggermiddleware.LogError(r.logger, err)
			}
			if len(deleteObjectResponse.GetFailed()) > 0 {
				loggermiddleware.LogError(
					r.logger,
					fmt.Errorf("failed to delete keys: %v", deleteObjectResponse.GetFailed()),
				)
			}
			wg.Done()
		}(bucket, keys)
	}

	c.JSON(http.StatusOK, ids)
	wg.Wait()
}

// Download is the request handler for /files/:id?alt=media request.
func (r *Router) Download(c *gin.Context) {
	// Get file ID from param.
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	isUserAllowed := r.HandleUserFilePermission(c, fileID)
	if !isUserAllowed {
		return
	}

	// Get the file meta from the file service
	fileMeta, err := r.fileClient.GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	filename := fileMeta.GetName()
	contentType := fileMeta.GetType()
	contentLength := fmt.Sprintf("%d", fileMeta.GetSize())

	downloadRequest := &dpb.DownloadRequest{
		Key:    fileMeta.GetKey(),
		Bucket: fileMeta.GetBucket(),
	}

	span, spanCtx := loggermiddleware.StartSpan(c.Request.Context(), "/download.Download/Download")
	defer span.End()

	stream, err := r.downloadClient.Download(spanCtx, downloadRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", contentLength)

	loggermiddleware.LogError(r.logger, HandleStream(c, stream))
}

// UpdateFile Updates single file.
// The function gets an id as a parameter and the partial file to update.
// It returns the updated file id.
func (r *Router) UpdateFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	if isUserAllowed := r.HandleUserFilePermission(c, fileID); !isUserAllowed {
		return
	}

	var pf partialFile
	if c.ShouldBindJSON(&pf) != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unexpected body format")),
		)

		return
	}

	// If the parent should be updated then check permissions for the new parent.
	if pf.Parent != nil {
		if isUserAllowed := r.HandleUserFilePermission(c, *pf.Parent); !isUserAllowed {
			return
		}
	}

	if err := r.handleUpdate(c, []string{fileID}, pf); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}
}

// UpdateFiles Updates many files with the same value.
// The function gets slice of ids and the partial file to update.
// It returns the updated file id's.
func (r *Router) UpdateFiles(c *gin.Context) {
	var body updateFilesRequest
	if c.ShouldBindJSON(&body) != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unexpected body format")),
		)

		return
	}

	// If the parent should be updated then check permissions for the new parent.
	if body.PartialFile.Parent != nil {
		if isUserAllowed := r.HandleUserFilePermission(c, *body.PartialFile.Parent); !isUserAllowed {
			return
		}
	}

	allowedIds := make([]string, 0, len(body.IDList))

	for _, id := range body.IDList {
		isUserAllowed, err := r.userFilePermission(c, id)

		if err != nil {
			loggermiddleware.LogError(r.logger, c.AbortWithError(int(status.Code(err)), err))
		}

		if isUserAllowed {
			allowedIds = append(allowedIds, id)
		}
	}

	if err := r.handleUpdate(c, allowedIds, body.PartialFile); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}
}

func (r *Router) handleUpdate(c *gin.Context, ids []string, pf partialFile) error {
	var parent *fpb.File_Parent

	if pf.Parent != nil {
		if *pf.Parent == "" {
			parent = &fpb.File_Parent{
				Parent: "null",
			}
		} else {
			parent = &fpb.File_Parent{
				Parent: *pf.Parent,
			}
		}
	}

	updatedData := &fpb.File{
		FileOrId: parent,
	}

	if len(ids) == 1 {
		updatedData.Name = pf.Name
		updatedData.Description = pf.Description
	}

	updateFilesResponse, err := r.fileClient.UpdateFiles(
		c.Request.Context(),
		&fpb.UpdateFilesRequest{
			IdList:      ids,
			PartialFile: updatedData,
		},
	)
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, updateFilesResponse.GetUpdated())
	return nil
}

// HandleStream streams the file bytes from stream to c.
func HandleStream(c *gin.Context, stream dpb.Download_DownloadClient) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			c.Status(http.StatusOK)

			// Returns error, need to decide how to handle
			if err := stream.CloseSend(); err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			if err := c.AbortWithError(httpStatusCode, err); err != nil {
				return err
			}

			if err := stream.CloseSend(); err != nil {
				return err
			}

			return nil
		}

		part := chunk.GetFile()
		if _, err := c.Writer.Write(part); err != nil {
			return err
		}
		c.Writer.Flush()
	}
}

type downloadZipBody struct {
	Files []string `json:"files"`
}

// DownloadZip is the request handler for downloading multiple files as a zip file.
func (r *Router) DownloadZip(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	body := downloadZipBody{}
	if err := c.ShouldBindJSON(&body); err != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unexpected body format")),
		)

		return
	}

	objects := make([]*fpb.File, 0, len(body.Files))
	for i := 0; i < len(body.Files); i++ {
		object, err := r.fileClient.GetFileByID(
			c.Request.Context(),
			&fpb.GetByFileByIDRequest{
				Id: body.Files[i],
			},
		)

		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		objects = append(objects, object)
	}

	archive := zip.NewWriter(c.Writer)
	defer archive.Close()

	buffer := make([]byte, download.PartSize)
	for _, object := range objects {
		zipit := func(object *fpb.File) error {
			stream, err := r.downloadClient.Download(c.Request.Context(), &dpb.DownloadRequest{
				Key:    object.GetKey(),
				Bucket: object.GetBucket(),
			})

			if err != nil {
				httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
				loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

				return err
			}

			readCloser := download.NewStreamReadCloser(stream)
			header := &zip.FileHeader{
				Name:               object.GetName(),
				Method:             zip.Deflate,
				UncompressedSize64: uint64(object.GetSize()),
			}
			header.SetModTime(time.Unix(object.GetUpdatedAt()/time.Microsecond.Microseconds(), 0))

			if hasStringSuffixInSlice(object.GetName(), standardExcludeCompressExtensions) ||
				hasPattern(standardExcludeCompressContentTypes, object.GetType()) {
				// We strictly disable compression for standard extensions/content-types.
				header.Method = zip.Store
			}

			writer, err := archive.CreateHeader(header)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return err
			}

			httpWriter := minioutil.WriteOnClose(writer)

			if _, err = io.CopyBuffer(httpWriter, readCloser, buffer); err != nil {
				httpWriter.Close()
				if !httpWriter.HasWritten() {
					httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
					loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
				}

				return err
			}

			if err = httpWriter.Close(); err != nil {
				if !httpWriter.HasWritten() {
					httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
					loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

					return err
				}
			}

			return nil
		}

		if object.GetType() != FolderContentType {
			if err := zipit(object); err != nil {
				httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
				loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

				return
			}
		}
	}
}

// Utility which returns if a string is present in the list.
// Comparison is case insensitive.
func hasStringSuffixInSlice(str string, list []string) bool {
	str = strings.ToLower(str)
	for _, v := range list {
		if strings.HasSuffix(str, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

// Returns true if any of the given wildcard patterns match the matchStr.
func hasPattern(patterns []string, matchStr string) bool {
	for _, pattern := range patterns {
		if ok := matchSimple(pattern, matchStr); ok {
			return true
		}
	}
	return false
}

// MatchSimple - finds whether the text matches/satisfies the pattern string.
// supports only '*' wildcard in the pattern.
// considers a file system path as a flat name space.
func matchSimple(pattern, name string) bool {
	if pattern == "" {
		return name == pattern
	}
	if pattern == "*" {
		return true
	}
	rname := make([]rune, 0, len(name))
	rpattern := make([]rune, 0, len(pattern))
	for _, r := range name {
		rname = append(rname, r)
	}
	for _, r := range pattern {
		rpattern = append(rpattern, r)
	}
	simple := true // Does only wildcard '*' match.
	return deepMatchRune(rname, rpattern, simple)
}

func deepMatchRune(str, pattern []rune, simple bool) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		default:
			if len(str) == 0 || str[0] != pattern[0] {
				return false
			}
		case '?':
			if len(str) == 0 && !simple {
				return false
			}
		case '*':
			return deepMatchRune(str, pattern[1:], simple) ||
				(len(str) > 0 && deepMatchRune(str[1:], pattern, simple))
		}
		str = str[1:]
		pattern = pattern[1:]
	}
	return len(str) == 0 && len(pattern) == 0
}

// userFilePermission gets a gin context holding the requesting user and the id of
// the file he's requesting. The function returns (true, nil) if the user is permitted
// to the file, (false, nil) if the user isn't permitted to it, and (false, error) where
// error is non-nil if an error occurred when calling FileServiceClient.IsAllowed.
func (r *Router) userFilePermission(c *gin.Context, fileID string) (bool, error) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		return false, nil
	}
	isAllowedResp, err := r.fileClient.IsAllowed(c.Request.Context(), &fpb.IsAllowedRequest{
		FileID: fileID,
		UserID: reqUser.ID,
	})

	if err != nil {
		return false, err
	}

	if !isAllowedResp.GetAllowed() {
		return false, nil
	}

	return true, nil
}

// HandleUserFilePermission gets a gin context and the id of the requested file.
// The function returns true if the user is permitted to operate on the file.
// The function returns false if the user isn't permitted to operate on it,
// The function also returns false if error if error occurred on r.userFilePermission
// and also log the error.
// It also handles error cases and Unauthorized operations by aborting with error/status.
func (r *Router) HandleUserFilePermission(c *gin.Context, fileID string) bool {
	isUserAllowed, err := r.userFilePermission(c, fileID)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		if err := c.AbortWithError(httpStatusCode, err); err != nil {
			return false
		}
	}

	if !isUserAllowed {
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	return true
}

// createGetFileResponse Creates a file grpc response to http response struct
func createGetFileResponse(file *fpb.File) (*getFileByIDResponse, error) {
	// Get file parent ID, if it doesn't exist check if it's an file object and get its ID.
	responseFile := &getFileByIDResponse{
		ID:          file.GetId(),
		Name:        file.GetName(),
		Type:        file.GetType(),
		Size:        file.GetSize(),
		Description: file.GetDescription(),
		OwnerID:     file.GetOwnerID(),
		Parent:      file.GetParent(),
		CreatedAt:   file.GetCreatedAt(),
		UpdatedAt:   file.GetUpdatedAt(),
	}

	// If file contains parent object instead of its id.
	fileParentObject := file.GetParentObject()
	if fileParentObject != nil {
		responseFile.Parent = fileParentObject.GetId()
	}

	return responseFile, nil
}
