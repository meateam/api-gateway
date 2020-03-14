package file

import (
	"context"
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
	"github.com/meateam/gotenberg-go-client/v6"
	ppb "github.com/meateam/permission-service/proto"
	spb "github.com/meateam/search-service/proto"
	upb "github.com/meateam/upload-service/proto"
	uspb "github.com/meateam/user-service/proto"
	minioutil "github.com/minio/minio/pkg/ioutil"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileParent is a constant for file parent parameter in a request.
	ParamFileParent = "parent"

	// ParamFileName is a constant for file name parameter in a request.
	ParamFileName = "name"

	// ParamFileType is a constant for file type parameter in a request.
	ParamFileType = "type"

	// ParamFileDescription is a constant for file description parameter in a request.
	ParamFileDescription = "description"

	// ParamFileSize is a constant for file size parameter in a request.
	ParamFileSize = "size"

	// ParamFileCreatedAt is a constant for file created at parameter in a request.
	ParamFileCreatedAt = "createdAt"

	// ParamFileUpdatedAt is a constant for file updated at parameter in a request.
	ParamFileUpdatedAt = "updatedAt"

	// FolderContentType is the custom content type of a folder.
	FolderContentType = "application/vnd.drive.folder"

	// QueryShareFiles is the querystring key for retrieving the files that were shared with the user.
	QueryShareFiles = "shares"

	// QueryFileDownloadPreview is the querystring key for
	// removing the content-disposition header from a file download.
	QueryFileDownloadPreview = "preview"

	// QueryPopulateOwner is the querystring key for populating the owner field
	// in the response object of a file.
	QueryPopulateOwner = "populateOwner"

	// QueryPopulateSharer is the querystring key for populating the sharer field
	// in the response object of a file.
	QueryPopulateSharer = "populateSharer"

	// OwnerRole is the owner role name when referred to as a permission.
	OwnerRole = "OWNER"

	// GetFileByIDRole is the role that is required of the authenticated requester to have to be
	// permitted to make the GetFileByID action.
	GetFileByIDRole = ppb.Role_READ

	// GetFilesByFolderRole is the role that is required of the authenticated requester to have to be
	// permitted to make the GetFilesByFolder action.
	GetFilesByFolderRole = ppb.Role_READ

	// DeleteFileByIDRole is the role that is required of the authenticated requester to have to be
	// permitted to make the DeleteFileByID action.
	DeleteFileByIDRole = ppb.Role_READ

	// DownloadRole is the role that is required of the authenticated requester to have to be
	// permitted to make the Download action.
	DownloadRole = ppb.Role_READ

	// UpdateFileRole is the role that is required of the authenticated requester to have to be
	// permitted to make the UpdateFile action.
	UpdateFileRole = ppb.Role_WRITE

	// UpdateFilesRole is the role that is required of the authenticated requester to have to be
	// permitted to make the UpdateFiles action.
	UpdateFilesRole = ppb.Role_WRITE

	// PdfMimeType is the mime type of a .pdf file.
	PdfMimeType = "application/pdf"

	// TextMimeType is the start of any file with a text mime type.
	TextMimeType = "text"

	// DocMimeType is the mime type of a .doc file.
	DocMimeType = "application/msword"

	// DocxMimeType is the mime type of a .docx file.
	DocxMimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	// XlsMimeType is the mime type of a .xls file.
	XlsMimeType = "application/vnd.ms-excel"

	// XlsxMimeType is the mime type of a .xlsx file.
	XlsxMimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	// PptMimeType is the mime type of a .ppt file.
	PptMimeType = "application/vnd.ms-powerpoint"

	// PptxMimeType is the mime type of a .pptx file.
	PptxMimeType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	// RtfMimeType is the mime type of a .rtf file.
	RtfMimeType = "application/rtf"

	// OdtMimeType is the mime type of a .odt file.
	OdtMimeType = "application/vnd.oasis.opendocument.text"

	// OdpMimeType is the mime type of a .odp file.
	OdpMimeType = "application/vnd.oasis.opendocument.presentation"
)

var (
	// TypesConvertableToPdf is a slice of the names of the mime types that can be converted to PDF and previewed.
	TypesConvertableToPdf = []string{
		DocMimeType,
		DocxMimeType,
		XlsMimeType,
		XlsxMimeType,
		PptMimeType,
		PptxMimeType,
		RtfMimeType,
		OdtMimeType,
		OdpMimeType,
	}
	
	// Some standard object extensions which we strictly dis-allow for compression.
	standardExcludeCompressExtensions = []string{".gz", ".bz2", ".rar", ".zip", ".7z", ".xz", ".mp4", ".mkv", ".mov"}

	// Some standard content-types which we strictly dis-allow for compression.
	standardExcludeCompressContentTypes = []string{"video/*", "audio/*", "application/zip", "application/x-gzip", "application/x-zip-compressed", " application/x-compress", "application/x-spoon"}
)

// Router is a structure that handles upload requests.
type Router struct {
	downloadClient   dpb.DownloadClient
	fileClient       fpb.FileServiceClient
	uploadClient     upb.UploadClient
	permissionClient ppb.PermissionClient
	searchClient     spb.SearchClient
	userClient       uspb.UsersClient
	gotenbergClient  *gotenberg.Client
	logger           *logrus.Logger
}

// Permission is a struct that describes a user's permission to a file.
type Permission struct {
	UserID  string `json:"userID,omitempty"`
	FileID  string `json:"fileID,omitempty"`
	Role    string `json:"role,omitempty"`
	Creator string `json:"creator,omitempty"`
}

// GetFileByIDResponse is a structure used for parsing fpb.File to a json file metadata response.
type GetFileByIDResponse struct {
	ID          string      `json:"id,omitempty"`
	Name        string      `json:"name,omitempty"`
	Type        string      `json:"type,omitempty"`
	Size        int64       `json:"size"`
	Description string      `json:"description,omitempty"`
	OwnerID     string      `json:"ownerId,omitempty"`
	Owner       *uspb.User  `json:"owner,omitempty"`
	Parent      string      `json:"parent,omitempty"`
	CreatedAt   int64       `json:"createdAt,omitempty"`
	UpdatedAt   int64       `json:"updatedAt,omitempty"`
	Role        string      `json:"role,omitempty"`
	Shared      bool        `json:"shared"`
	Sharer      *uspb.User  `json:"sharer,omitempty"`
	Permission  *Permission `json:"permission,omitempty"`
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
	Float       bool    `json:"float,omitempty"`
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
	permissionConn *grpc.ClientConn,
	searchConn *grpc.ClientConn,
	userConn *grpc.ClientConn,
	gotenbergClient *gotenberg.Client,
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
	r.permissionClient = ppb.NewPermissionClient(permissionConn)
	r.searchClient = spb.NewSearchClient(searchConn)
	r.userClient = uspb.NewUsersClient(userConn)
	r.gotenbergClient = gotenbergClient

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/files", r.GetFilesByFolder)
	rg.GET("/files/:id", r.GetFileByID)
	rg.GET("/files/:id/ancestors", r.GetFileAncestors)
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

	userFilePermission, foundPermission := r.HandleUserFilePermission(c, fileID, GetFileByIDRole)
	if userFilePermission == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
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

	c.JSON(http.StatusOK, CreateGetFileResponse(file, userFilePermission, foundPermission))
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

	if _, exists := c.GetQuery(QueryShareFiles); exists {
		r.GetSharedFiles(c)
		return
	}

	filesParent := c.Query(ParamFileParent)
	if userFilePermission, _ := r.HandleUserFilePermission(
		c,
		filesParent,
		GetFilesByFolderRole); userFilePermission == "" {
		return
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
		Float:       false,
	}

	fileOwner := reqUser.ID
	if filesParent != "" {
		fileOwner = ""
	}

	// Use the id of the owner of parent to get the folder's files.
	filesResp, err := r.fileClient.GetFilesByFolder(
		c.Request.Context(),
		&fpb.GetFilesByFolderRequest{OwnerID: fileOwner, FolderID: filesParent, QueryFile: &fileFilter},
	)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	files := filesResp.GetFiles()
	responseFiles := make([]*GetFileByIDResponse, 0, len(files))
	for _, file := range files {
		userFilePermission, foundPermission, err := CheckUserFilePermission(c.Request.Context(),
			r.fileClient,
			r.permissionClient,
			reqUser.ID,
			file.GetId(),
			GetFilesByFolderRole)
		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		if userFilePermission != "" {
			responseFiles = append(responseFiles, CreateGetFileResponse(file, userFilePermission, foundPermission))
		}
	}

	// Populate sharer field if ?populateSharer is found in the request's query.
	if _, exists := c.GetQuery(QueryPopulateSharer); exists {
		if err := r.populateFileSharer(c.Request.Context(), responseFiles); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}
	}

	// Populate owner field if ?populateOwner is found in the request's query.
	if _, exists := c.GetQuery(QueryPopulateOwner); exists {
		if err := r.populateFileOwner(c.Request.Context(), responseFiles); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}
	}

	c.JSON(http.StatusOK, responseFiles)
}

// GetSharedFiles is the request handler for GET /files?shares
func (r *Router) GetSharedFiles(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	permissions, err := r.permissionClient.GetUserPermissions(
		c.Request.Context(),
		&ppb.GetUserPermissionsRequest{UserID: reqUser.ID},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	files := make([]*GetFileByIDResponse, 0, len(permissions.GetPermissions()))
	for _, permission := range permissions.GetPermissions() {
		file, err := r.fileClient.GetFileByID(c.Request.Context(),
			&fpb.GetByFileByIDRequest{Id: permission.GetFileID()})
		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		if file.GetOwnerID() != reqUser.ID {
			userPermission := &ppb.PermissionObject{
				FileID:  permission.GetFileID(),
				UserID:  reqUser.ID,
				Role:    permission.GetRole(),
				Creator: permission.GetCreator(),
			}

			files = append(
				files,
				CreateGetFileResponse(file, permission.GetRole().String(), userPermission),
			)
		}
	}

	if _, exists := c.GetQuery(QueryPopulateSharer); exists {
		if err := r.populateFileSharer(c.Request.Context(), files); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}
	}

	c.JSON(http.StatusOK, files)
}

// DeleteFileByID is the request handler for DELETE /files/:id request.
func (r *Router) DeleteFileByID(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	if role, _ := r.HandleUserFilePermission(c, fileID, DeleteFileByIDRole); role == "" {
		return
	}

	ids, err := DeleteFile(
		c.Request.Context(),
		r.logger,
		r.fileClient,
		r.uploadClient,
		r.searchClient,
		r.permissionClient,
		fileID,
		reqUser.ID)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	c.JSON(http.StatusOK, ids)
}

// Download is the request handler for /files/:id?alt=media request.
func (r *Router) Download(c *gin.Context) {
	// Get file ID from param.
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	if role, _ := r.HandleUserFilePermission(c, fileID, DownloadRole); role == "" {
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

	preview, ok := c.GetQuery(QueryFileDownloadPreview)
	if ok && preview != "false" {
		loggermiddleware.LogError(r.logger, r.HandlePreview(c, fileMeta, stream))

		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Length", contentLength)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename="+filename)

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

	if role, _ := r.HandleUserFilePermission(c, fileID, UpdateFileRole); role == "" {
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
		if role, _ := r.HandleUserFilePermission(c, *pf.Parent, UpdateFileRole); role == "" {
			return
		}
	}

	if err := r.handleUpdate(c, []string{fileID}, pf); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}
}

// GetFileAncestors returns an array of the requested file ancestors.
// The function gets an id.
// It returns the updated file id's.
func (r *Router) GetFileAncestors(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	userFilePermission, _ := r.HandleUserFilePermission(c, fileID, GetFileByIDRole)
	if userFilePermission == "" {
		return
	}

	res, err := r.fileClient.GetAncestors(c.Request.Context(), &fpb.GetAncestorsRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ancestors := res.GetAncestors()

	type permissionRole struct {
		permission *ppb.PermissionObject
		role       string
	}

	ancestorsPermissionsMap := make(map[string]permissionRole, len(ancestors))

	var firstPermittedFileIndex int
	for firstPermittedFileIndex = 0; firstPermittedFileIndex < len(ancestors); firstPermittedFileIndex++ {
		userFilePermission, foundPermission, err := CheckUserFilePermission(
			c.Request.Context(),
			r.fileClient,
			r.permissionClient,
			reqUser.ID,
			ancestors[firstPermittedFileIndex],
			GetFileByIDRole,
		)

		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		ancestorsPermissionsMap[ancestors[firstPermittedFileIndex]] = permissionRole{
			permission: foundPermission,
			role:       userFilePermission,
		}

		if userFilePermission != "" {
			break
		}
	}

	permittedAncestors := ancestors[firstPermittedFileIndex:]

	populatedPermittedAncestors := make([]*GetFileByIDResponse, 0, len(permittedAncestors))

	for i := 0; i < len(permittedAncestors); i++ {
		file, err := r.fileClient.GetFileByID(
			c.Request.Context(),
			&fpb.GetByFileByIDRequest{Id: permittedAncestors[i]},
		)
		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		ancestorPermissionRole := ancestorsPermissionsMap[permittedAncestors[i]]
		populatedPermittedAncestors = append(
			populatedPermittedAncestors,
			CreateGetFileResponse(file, ancestorPermissionRole.role, ancestorPermissionRole.permission))
	}

	c.JSON(http.StatusOK, populatedPermittedAncestors)
}

// UpdateFiles Updates many files with the same value.
// The function gets slice of ids and the partial file to update.
// It returns the updated file id's.
func (r *Router) UpdateFiles(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)

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
		if role, _ := r.HandleUserFilePermission(c, *body.PartialFile.Parent, UpdateFilesRole); role == "" {
			return
		}
	}

	allowedIds := make([]string, 0, len(body.IDList))

	for _, id := range body.IDList {
		userFilePermission, _, err := CheckUserFilePermission(c.Request.Context(),
			r.fileClient,
			r.permissionClient,
			reqUser.ID,
			id,
			UpdateFilesRole)
		if err != nil {
			loggermiddleware.LogError(r.logger, c.AbortWithError(int(status.Code(err)), err))
		}

		if userFilePermission != "" {
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
	var sParent *spb.File_Parent
	if pf.Parent != nil {
		if *pf.Parent == "" {
			parent = &fpb.File_Parent{
				Parent: "null",
			}
			sParent = &spb.File_Parent{
				Parent: "null",
			}
		} else {
			parent = &fpb.File_Parent{
				Parent: *pf.Parent,
			}
			sParent = &spb.File_Parent{
				Parent: *pf.Parent,
			}
		}
	}

	updatedData := &fpb.File{
		FileOrId: parent,
		Float:    pf.Float,
	}

	sUpdatedData := &spb.File{
		FileOrId: sParent,
	}

	if len(ids) == 1 {
		updatedData.Name = pf.Name
		sUpdatedData.Name = pf.Name

		updatedData.Description = pf.Description
		sUpdatedData.Description = pf.Description
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

	for _, id := range ids {
		sUpdatedData.Id = id
		if _, err := r.searchClient.Update(c.Request.Context(), sUpdatedData); err != nil {
			r.logger.Errorf("failed to update file %s in searchService", id)
		}
	}

	c.JSON(http.StatusOK, updateFilesResponse.GetFailedFiles())
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
			header.SetModTime(time.Unix(object.GetUpdatedAt()/time.Second.Milliseconds(), 0))

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

// CheckUserFilePermission checks if userID is permitted to fileID with the wanted role.
// The function returns the role name if the user is permitted to the file,
// the permission if the user was shared, and non-nil err if any encountered.
// If no permitted then role found role would be "".
// If user was shared then permission would be non-nil.
func CheckUserFilePermission(ctx context.Context,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	userID string,
	fileID string,
	role ppb.Role) (string, *ppb.PermissionObject, error) {
	if userID == "" {
		return "", nil, fmt.Errorf("userID is required")
	}

	// Everyone is permitted to their root, since all actions on root are authenticated,
	// and it's impossible to create a permission for root (aka sharing a user's whole drive).
	if fileID == "" {
		return OwnerRole, nil, nil
	}

	// Get the file's metadata.
	file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		return "", nil, err
	}

	// Check if the owner of the current file is userID, if so then he's permitted.
	if file.GetOwnerID() == userID {
		return OwnerRole, nil, nil
	}

	// Go up the hirarchy searching for a permission for userID to fileID with role.
	// Fetch fileID's parents, each at a time, and check permission to each parent.
	// If reached a parent that userID isn't permitted to then return with error,
	// If reached a parent that userID is permitted to then return true with nil error.
	// If any error encountered then return false and the encountered error.
	currentFile := fileID
	for {
		// If reached the root and didn't find a permission then userID is not permitted to fileID.
		if currentFile == "" {
			return "", nil, nil
		}

		// Check if the user has an existing permission and is permitted to currentFile with the wanted role.
		isPermitted, err := permissionClient.IsPermitted(ctx,
			&ppb.IsPermittedRequest{FileID: currentFile, UserID: userID, Role: role})

		// If an error occurred which is NOT grpc's NotFound error which
		// indicates that the permission doesn't not exist.
		if err != nil && status.Code(err) != codes.NotFound {
			return "", nil, err
		}

		// If no error received and user isn't permitted.
		if !isPermitted.GetPermitted() && err == nil {
			return "", nil, nil
		}

		// If userID is permitted with the wanted role then return the role that the user has for the file.
		if isPermitted.GetPermitted() {
			permission, err := permissionClient.GetPermission(
				ctx,
				&ppb.GetPermissionRequest{
					FileID: currentFile,
					UserID: userID,
				},
			)

			if err != nil {
				return "", nil, err
			}

			return permission.GetRole().String(), permission, nil
		}

		// Get the current file's metadata.
		file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: currentFile})
		if err != nil {
			return "", nil, err
		}

		// Repeat for the file's parent.
		currentFile = file.GetParent()
	}
}

// CreatePermission creates permission in permission service only if userID has
// ppb.Role_WRITE permission to permission.FileID.
func CreatePermission(ctx context.Context,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	userID string,
	permission ppb.PermissionObject) error {
	// Check if userID has ppb.Role_WRITE permission to permission.FileID.
	userFilePermission, _, err := CheckUserFilePermission(ctx,
		fileClient,
		permissionClient,
		userID,
		permission.GetFileID(),
		ppb.Role_WRITE)
	if err != nil {
		return fmt.Errorf("failed creating permission: %v", err)
	}

	if userFilePermission == "" {
		return fmt.Errorf("failed creating permission: user %s is not the permitted to file %s",
			userID, permission.GetFileID())
	}

	if permission.GetRole() == ppb.Role_NONE {
		return fmt.Errorf("failed creating permission: cannot set Role_NONE to a user's permission")
	}

	createPermissionRequest := ppb.CreatePermissionRequest{
		FileID:  permission.GetFileID(),
		UserID:  permission.GetUserID(),
		Role:    permission.GetRole(),
		Creator: permission.GetCreator(),
	}
	_, err = permissionClient.CreatePermission(ctx, &createPermissionRequest)
	if err != nil {
		return fmt.Errorf("failed creating permission: %v", err)
	}

	return nil
}

// HandleUserFilePermission gets a gin context and the id of the requested file,
// returns true if the user is permitted to operate on the file.
// Returns false if the user isn't permitted to operate on it,
// Returns false if any error occurred and logs the error.
func (r *Router) HandleUserFilePermission(
	c *gin.Context,
	fileID string,
	role ppb.Role) (string, *ppb.PermissionObject) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return "", nil
	}

	userFilePermission, foundPermission, err := CheckUserFilePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		fileID,
		role)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return "", nil
	}

	if userFilePermission == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return userFilePermission, foundPermission
}

// CreateGetFileResponse Creates a file grpc response to http response struct.
func CreateGetFileResponse(file *fpb.File, role string, permission *ppb.PermissionObject) *GetFileByIDResponse {
	if file == nil {
		return nil
	}

	// Get file parent ID, if it doesn't exist check if it's an file object and get its ID.
	responseFile := &GetFileByIDResponse{
		ID:          file.GetId(),
		Name:        file.GetName(),
		Type:        file.GetType(),
		Size:        file.GetSize(),
		Description: file.GetDescription(),
		OwnerID:     file.GetOwnerID(),
		Parent:      file.GetParent(),
		CreatedAt:   file.GetCreatedAt(),
		UpdatedAt:   file.GetUpdatedAt(),
		Role:        role,
		Shared:      false,
	}

	if permission != nil {
		responseFile.Shared = true
		responseFile.Permission = &Permission{
			UserID:  permission.GetUserID(),
			FileID:  permission.GetFileID(),
			Role:    permission.GetRole().String(),
			Creator: permission.GetCreator(),
		}
	}

	// If file contains parent object instead of its id.
	fileParentObject := file.GetParentObject()
	if fileParentObject != nil {
		responseFile.Parent = fileParentObject.GetId()
	}

	return responseFile
}

// HandlePreview writes a PDF of the file to the response, if the file isn't a PDF already
// and can be converted to a PDF, then the file would be converted to a PDF and
// the converted file will be written to the response, instead of the raw file.
func (r *Router) HandlePreview(c *gin.Context, file *fpb.File, stream dpb.Download_DownloadClient) error {
	filename := file.GetName()
	contentType := file.GetType()
	contentLength := strconv.FormatInt(file.GetSize(), 10)

	// File is already a PDF, no need to convert it.
	if contentType == PdfMimeType || strings.HasPrefix(contentType, TextMimeType) {
		c.Header("Content-Type", contentType)
		c.Header("Content-Length", contentLength)

		return HandleStream(c, stream)
	}

	// Convert the file to PDF.
	streamReader := download.NewStreamReadCloser(stream)

	// IMPORTANT: Must use a buffer that its size is at least download.PartSize, otherwise data loss
	// would occure.
	buf := make([]byte, download.PartSize)
	convertRequest, err := gotenberg.NewOfficeRequestWithBuffer(filename, streamReader, buf)
	if err != nil {
		return err
	}

	convertRequest.ResultFilename(filename)

	defer streamReader.Close()

	// Send the file for PDF conversion.
	resp, err := r.gotenbergClient.Post(convertRequest)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("failed converting file with gotenberg with status: %v", resp.Status)
		loggermiddleware.LogError(r.logger, c.AbortWithError(resp.StatusCode, err))

		return err
	}

	c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	c.Header("Content-Type", resp.Header.Get("Content-Type"))

	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusInternalServerError, err))

		return err
	}

	c.Status(http.StatusOK)

	return nil
}

// populateFileOwner populates the owner field with the info of the user who gave the
// permission to each of the files in the given files slice.
// Implementation is making len(files) concurrent requests with r.userClient.
func (r *Router) populateFileOwner(ctx context.Context, files []*GetFileByIDResponse) error {
	usersChan := make(chan struct {
		user  *uspb.User
		index int
		err   error
	}, len(files))
	wg := sync.WaitGroup{}
	for i := 0; i < len(files); i++ {
		getUserByIDRequest := &uspb.GetByIDRequest{
			Id: files[i].OwnerID,
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			user, err := r.userClient.GetUserByID(ctx, getUserByIDRequest)
			if err != nil {
				usersChan <- struct {
					user  *uspb.User
					index int
					err   error
				}{
					user:  nil,
					index: index,
					err:   err,
				}

				return
			}

			usersChan <- struct {
				user  *uspb.User
				index int
				err   error
			}{
				user:  user.GetUser(),
				index: index,
				err:   err,
			}
		}(i)
	}

	wg.Wait()
	close(usersChan)

	for userStruct := range usersChan {
		if userStruct.err != nil {
			return userStruct.err
		}

		files[userStruct.index].Owner = userStruct.user
	}

	return nil
}

// populateFileSharer populates the sharer field with the info of the user who gave the
// permission to each of the files in the given files slice.
// Implementation is making len(files) concurrent requests with r.userClient.
func (r *Router) populateFileSharer(ctx context.Context, files []*GetFileByIDResponse) error {
	usersChan := make(chan struct {
		user  *uspb.User
		index int
		err   error
	}, len(files))
	wg := sync.WaitGroup{}
	for i := 0; i < len(files); i++ {
		if files[i].Permission == nil {
			continue
		}

		getUserByIDRequest := &uspb.GetByIDRequest{
			Id: files[i].Permission.Creator,
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			user, err := r.userClient.GetUserByID(ctx, getUserByIDRequest)
			if err != nil {
				usersChan <- struct {
					user  *uspb.User
					index int
					err   error
				}{
					user:  nil,
					index: index,
					err:   err,
				}

				return
			}

			usersChan <- struct {
				user  *uspb.User
				index int
				err   error
			}{
				user:  user.GetUser(),
				index: index,
				err:   err,
			}
		}(i)
	}

	wg.Wait()
	close(usersChan)

	for userStruct := range usersChan {
		if userStruct.err != nil {
			return userStruct.err
		}

		files[userStruct.index].Sharer = userStruct.user
	}

	return nil
}

// IsFileConvertableToPdf returns true if contentType can be converted to a PDF file, false otherwise.
func IsFileConvertableToPdf(contentType string) bool {
	for _, v := range TypesConvertableToPdf {
		if contentType == v {
			return true
		}
	}

	return false
}
