package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	auth "github.com/meateam/api-gateway/oauth"
	oauth "github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/api-gateway/utils"
	"github.com/meateam/download-service/download"
	dpb "github.com/meateam/download-service/proto"
	drp "github.com/meateam/dropbox-service/proto/dropbox"
	fpb "github.com/meateam/file-service/proto/file"
	"github.com/meateam/gotenberg-go-client/v6"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	ppb "github.com/meateam/permission-service/proto"
	spb "github.com/meateam/search-service/proto"
	upb "github.com/meateam/upload-service/proto"
	"github.com/sirupsen/logrus"
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

	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"

	// ParamFileUpdatedAt is a constant for file updated at parameter in a request.
	ParamFileUpdatedAt = "updatedAt"

	// ParamPageNum is a constant for the requested page num in the pagination.
	ParamPageNum = "pageNum"

	// ParamPageSize is a constant for the requested page size in the pagination.
	ParamPageSize = "pageSize"

	// QueryShareFiles is the querystring key for retrieving the files that were shared with the user.
	QueryShareFiles = "shares"

	// QueryAppID is a constant for queryAppId parameter in a request.
	// If exists, the files returned will only belong to the app of QueryAppID.
	QueryAppID = "appId"

	// QueryFileDownloadPreview is the querystring key for
	// removing the content-disposition header from a file download.
	QueryFileDownloadPreview = "preview"

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

	// FileIDIsRequiredMessage is the error message for missing fileID
	FileIDIsRequiredMessage = "fileID is required"

	// ContentDispositionHeader content-disposition header name.
	ContentDispositionHeader = "Content-Disposition"

	// FolderContentType is the custom content type of a folder.
	FolderContentType = "application/vnd.drive.folder"
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

	// AllowedAllOperationsApps are the applications which are allowed to do any operation
	// open to external apps on files which are not theirs
	AllowedAllOperationsApps = []string{oauth.DriveAppID}

	// AllowedDownloadApps are the applications which are only allowed to download
	// files which are not theirs
	AllowedDownloadApps = []string{oauth.DriveAppID, oauth.DropboxAppID, oauth.CargoAppID}

	// Some standard object extensions which we strictly dis-allow for compression.
	standardExcludeCompressExtensions = []string{".gz", ".bz2", ".rar", ".zip", ".7z", ".xz", ".mp4", ".mkv", ".mov"}

	// Some standard content-types which we strictly dis-allow for compression.
	standardExcludeCompressContentTypes = []string{"video/*", "audio/*", "application/zip", "application/x-gzip",
		"application/x-zip-compressed", " application/x-compress", "application/x-spoon"}
)

// Router is a structure that handles upload requests.
type Router struct {
	// DownloadClientFactory
	downloadClient factory.DownloadClientFactory

	// FileClientFactory
	fileClient factory.FileClientFactory

	// UploadClientFactory
	uploadClient factory.UploadClientFactory

	// PermissionClientFactory
	permissionClient factory.PermissionClientFactory

	// DropboxClientFactory
	dropboxClient factory.DropboxClientFactory

	// SearchClientFactory
	searchClient factory.SearchClientFactory

	gotenbergClient *gotenberg.Client
	oAuthMiddleware *oauth.Middleware
	logger          *logrus.Logger
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
	Parent      string      `json:"parent,omitempty"`
	CreatedAt   int64       `json:"createdAt,omitempty"`
	UpdatedAt   int64       `json:"updatedAt,omitempty"`
	Role        string      `json:"role,omitempty"`
	Shared      bool        `json:"shared"`
	Permission  *Permission `json:"permission,omitempty"`
	IsExternal  bool        `json:"isExternal"`
	AppID       string      `json:"appID,omitempty"`
}

// GetSharedFilesResponse is a structure used for the response of getSharedFiles.
type GetSharedFilesResponse struct {
	Files     []*GetFileByIDResponse `json:"files"`
	PageNum   int64                  `json:"pageNum"`
	ItemCount int64                  `json:"itemCount"`
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
	fileConn *grpcPoolTypes.ConnPool,
	downloadConn *grpcPoolTypes.ConnPool,
	uploadConn *grpcPoolTypes.ConnPool,
	permissionConn *grpcPoolTypes.ConnPool,
	dropboxConn *grpcPoolTypes.ConnPool,
	searchConn *grpcPoolTypes.ConnPool,
	gotenbergClient *gotenberg.Client,
	oAuthMiddleware *oauth.Middleware,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.fileClient = func() fpb.FileServiceClient {
		return fpb.NewFileServiceClient((*fileConn).Conn())
	}

	r.downloadClient = func() dpb.DownloadClient {
		return dpb.NewDownloadClient((*downloadConn).Conn())
	}

	r.uploadClient = func() upb.UploadClient {
		return upb.NewUploadClient((*uploadConn).Conn())
	}

	r.permissionClient = func() ppb.PermissionClient {
		return ppb.NewPermissionClient((*permissionConn).Conn())
	}

	r.dropboxClient = func() drp.DropboxClient {
		return drp.NewDropboxClient((*dropboxConn).Conn())
	}

	r.searchClient = func() spb.SearchClient {
		return spb.NewSearchClient((*searchConn).Conn())
	}

	r.gotenbergClient = gotenbergClient

	r.oAuthMiddleware = oAuthMiddleware

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	checkGetFileScope := r.oAuthMiddleware.AuthorizationScopeMiddleware(auth.GetFileScope)
	checkDeleteFileScope := r.oAuthMiddleware.AuthorizationScopeMiddleware(auth.DeleteScope)

	rg.GET("/files", checkGetFileScope, r.GetFilesByFolder)
	rg.GET("/files/:id", checkGetFileScope, r.GetFileByID)
	rg.GET("/files/:id/ancestors", r.GetFileAncestors)
	rg.DELETE("/files/:id", checkDeleteFileScope, r.DeleteFileByID)
	rg.PUT("/files/:id", r.UpdateFile)
	rg.PUT("/files", r.UpdateFiles)
}

// GetFileByID is the request handler for GET /files/:id
func (r *Router) GetFileByID(c *gin.Context) {
	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.String(http.StatusBadRequest, FileIDIsRequiredMessage)
		return
	}

	err := validateAppID(c, fileID, r.fileClient(), AllowedDownloadApps)
	if err != nil {
		loggermiddleware.LogError(r.logger, err)
		return
	}

	alt := c.Query("alt")
	if alt == "media" {
		canDownload := r.oAuthMiddleware.ValidateRequiredScope(c, oauth.DownloadScope)

		if !canDownload {
			loggermiddleware.LogError(r.logger, c.AbortWithError(
				http.StatusForbidden,
				fmt.Errorf("required scope '%s' is not supplied", oauth.DownloadScope),
			))
			return
		}
		r.Download(c)

		return
	}

	userFilePermission, foundPermission := r.HandleUserFilePermission(c, fileID, GetFileByIDRole)
	if userFilePermission == "" {
		if !r.HandleUserFilePermit(c, fileID, GetFileByIDRole) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	getFileByIDRequest := &fpb.GetByFileByIDRequest{Id: fileID}
	file, err := r.fileClient().GetFileByID(c.Request.Context(), getFileByIDRequest)
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

// GetFilesByFolder is the request handler for GET /files request.
func (r *Router) GetFilesByFolder(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filesParent := c.Query(ParamFileParent)
	err := validateAppID(c, filesParent, r.fileClient(), AllowedAllOperationsApps)
	if err != nil {
		loggermiddleware.LogError(r.logger, err)
		return
	}

	// Get the application ID of the app which sent the request.
	// This was saved in the oauth middleware.
	appID := c.Value(oauth.ContextAppKey).(string)
	queryAppID := appID

	// Check if a specific app was requested by the drive.
	// Other apps are not permitted to do so.
	if stringInSlice(appID, AllowedAllOperationsApps) {
		queryAppID = c.Query(QueryAppID)
	}

	// Check if client requested all files shared with him.
	if _, exists := c.GetQuery(QueryShareFiles); exists {
		// Only AllowedAllOperationsApps can access GetSharedFiles.
		// In the future - we may allow other apps to get the
		// shared files which belong to them.
		if !stringInSlice(appID, AllowedAllOperationsApps) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		r.GetSharedFiles(c, queryAppID)
		return
	}

	userFilePermission, _ := r.HandleUserFilePermission(
		c,
		filesParent,
		GetFilesByFolderRole)

	if userFilePermission == "" {
		if !r.HandleUserFilePermit(c, filesParent, GetFilesByFolderRole) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	paramMap := queryParamsToMap(c, ParamFileName, ParamFileType, ParamFileDescription, ParamFileSize,
		ParamFileCreatedAt, ParamFileUpdatedAt)

	fileFilter := fpb.File{
		Name:        paramMap[ParamFileName],
		Type:        paramMap[ParamFileType],
		Description: paramMap[ParamFileDescription],
		Size:        utils.StringToInt64(paramMap[ParamFileSize]),
		CreatedAt:   utils.StringToInt64(paramMap[ParamFileCreatedAt]),
		UpdatedAt:   utils.StringToInt64(paramMap[ParamFileUpdatedAt]),
		Float:       false,
		AppID:       queryAppID,
	}

	fileOwner := reqUser.ID
	if filesParent != "" {
		fileOwner = ""
	}

	// Use the id of the owner of parent to get the folder's files.
	filesResp, err := r.fileClient().GetFilesByFolder(
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
			r.fileClient(),
			r.permissionClient(),
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

	c.JSON(http.StatusOK, responseFiles)
}

// GetSharedFiles is the request handler for GET /files?shares.
// Currently, can only be requested by AllowedAllOperationsApps.
// queryAppID is the specific app requested by the application.
func (r *Router) GetSharedFiles(c *gin.Context, queryAppID string) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	pageNum := utils.StringToInt64(c.Query(ParamPageNum))
	pageSize := utils.StringToInt64(c.Query(ParamPageSize))

	// Return a page of all shared files' permissions which belong to the user,
	// filtered by appID. If queryAppID = "", it will not filter by apps
	permissions, err := r.permissionClient().GetUserPermissions(
		c.Request.Context(),
		&ppb.GetUserPermissionsRequest{
			UserID:   reqUser.ID,
			AppID:    queryAppID,
			PageNum:  pageNum,
			PageSize: pageSize,
			IsShared: true},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	// Go over the permissions and get the files metadata related to them
	files := make([]*GetFileByIDResponse, 0, len(permissions.GetPermissions()))
	for _, permission := range permissions.GetPermissions() {
		file, err := r.fileClient().GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: permission.GetFileID()})
		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		// Filter files which belong to the requesting user.
		// The creator of the permission is not necessarily the owner of the file!
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

	sharedFilesResponse := &GetSharedFilesResponse{
		Files:     files,
		PageNum:   permissions.PageNum,
		ItemCount: permissions.ItemCount,
	}

	c.JSON(http.StatusOK, sharedFilesResponse)
}

// DeleteFileByID is the request handler for DELETE /files/:id request.
func (r *Router) DeleteFileByID(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.String(http.StatusBadRequest, FileIDIsRequiredMessage)
		return
	}

	err := validateAppID(c, fileID, r.fileClient(), AllowedAllOperationsApps)
	if err != nil {
		loggermiddleware.LogError(r.logger, err)
		return
	}

	if role, _ := r.HandleUserFilePermission(c, fileID, DeleteFileByIDRole); role == "" {
		return
	}

	ids, err := DeleteFile(
		c.Request.Context(),
		r.logger,
		r.fileClient(),
		r.uploadClient(),
		r.searchClient(),
		r.permissionClient(),
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
	fileID := c.Param(ParamFileID)

	role, _ := r.HandleUserFilePermission(c, fileID, GetFileByIDRole)

	if role == "" {
		if !r.HandleUserFilePermit(c, fileID, DownloadRole) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	// Get the file meta from the file service
	fileMeta, err := r.fileClient().GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	filename := fileMeta.GetName()
	contentType := fileMeta.GetType()

	if contentType == FolderContentType {
		r.downloadFolder(c, fileMeta)

		return
	}

	contentLength := fmt.Sprintf("%d", fileMeta.GetSize())

	downloadRequest := &dpb.DownloadRequest{
		Key:    fileMeta.GetKey(),
		Bucket: fileMeta.GetBucket(),
	}

	span, spanCtx := loggermiddleware.StartSpan(c.Request.Context(), "/download.Download/Download")
	defer span.End()

	stream, err := r.downloadClient().Download(spanCtx, downloadRequest)
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
	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.String(http.StatusBadRequest, FileIDIsRequiredMessage)
		return
	}

	err := validateAppID(c, fileID, r.fileClient(), AllowedAllOperationsApps)
	if err != nil {
		loggermiddleware.LogError(r.logger, err)
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
	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.String(http.StatusBadRequest, FileIDIsRequiredMessage)
		return
	}

	err := validateAppID(c, fileID, r.fileClient(), AllowedAllOperationsApps)
	if err != nil {
		loggermiddleware.LogError(r.logger, err)
		return
	}

	userFilePermission, _ := r.HandleUserFilePermission(c, fileID, GetFileByIDRole)
	if userFilePermission == "" {
		return
	}

	res, err := r.fileClient().GetAncestors(c.Request.Context(), &fpb.GetAncestorsRequest{Id: fileID})
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
			r.fileClient(),
			r.permissionClient(),
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
		file, err := r.fileClient().GetFileByID(
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
			r.fileClient(),
			r.permissionClient(),
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

	updateFilesResponse, err := r.fileClient().UpdateFiles(
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
		if _, err := r.searchClient().Update(c.Request.Context(), sUpdatedData); err != nil {
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

	// Go up the hierarchy searching for a permission for userID to fileID with role.
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

// CheckUserFileTransfer checks if userID is has a transfer to fileID.
// The function returns true if the user has a transfer to the file and nil error,
// otherwise false and non-nil err if any encountered.
func CheckUserFileTransfer(ctx context.Context,
	dropboxClient drp.DropboxClient,
	userID string,
	fileID string,
	role ppb.Role) (bool, error) {

	// Permits have only READ roles
	if role != ppb.Role_READ {
		return false, nil
	}

	hasTransferRes, err := dropboxClient.HasTransfer(ctx, &drp.HasTransferRequest{FileID: fileID, UserID: userID})
	if err != nil {
		return false, err
	}

	HasTransfer := hasTransferRes.GetHasTransfer()
	return HasTransfer, nil
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
		AppID:   permission.GetAppID(),
		Role:    permission.GetRole(),
		Creator: permission.GetCreator(),
	}
	_, err = permissionClient.CreatePermission(ctx, &createPermissionRequest)
	if err != nil {
		return fmt.Errorf("failed creating permission: %v", err)
	}

	return nil
}

// HandleUserFilePermission gets the id of the requested file, and the required role.
// Returns the user role as a string, and the permission if the user is permitted
// to operate on the file, and `"", nil` if not.
func (r *Router) HandleUserFilePermission(
	c *gin.Context,
	fileID string,
	role ppb.Role) (string, *ppb.PermissionObject) {
	reqUser := user.ExtractRequestUser(c)

	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return "", nil
	}

	userStringRole, foundPermission, err := CheckUserFilePermission(c.Request.Context(),
		r.fileClient(),
		r.permissionClient(),
		reqUser.ID,
		fileID,
		role)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return "", nil
	}

	if userStringRole == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return userStringRole, foundPermission
}

// HandleUserFilePermit gets a gin context and the id of the requested file,
// returns true if the user is permitted to operate on the file.
// Returns false if the user isn't permitted to operate on it,
// Returns false if any error occurred and logs the error.
func (r *Router) HandleUserFilePermit(
	c *gin.Context,
	fileID string,
	role ppb.Role) bool {

	reqUser := user.ExtractRequestUser(c)

	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return false
	}

	isPermitted, err := CheckUserFileTransfer(c.Request.Context(),
		r.dropboxClient(),
		reqUser.ID,
		fileID,
		role)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return false
	}

	if !isPermitted {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return isPermitted
}

// CreateGetFileResponse Creates a file grpc response to http response struct.
func CreateGetFileResponse(file *fpb.File, role string, permission *ppb.PermissionObject) *GetFileByIDResponse {
	if file == nil {
		return nil
	}

	isExternal := user.IsExternalUser(file.OwnerID)

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
		IsExternal:  isExternal,
		AppID:       file.GetAppID(),
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
	// would occur.
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

// IsFileConvertableToPdf returns true if contentType can be converted to a PDF file, false otherwise.
func IsFileConvertableToPdf(contentType string) bool {
	for _, v := range TypesConvertableToPdf {
		if contentType == v {
			return true
		}
	}

	return false
}

// validateAppID returns an error if the app cannot do an operation on the file, otherwise, nil.
// The allowedApps are permitted to do any operation.
func validateAppID(ctx *gin.Context, fileID string, fileClient fpb.FileServiceClient, allowedApps []string) error {
	appID := ctx.Value(oauth.ContextAppKey).(string)

	// Check if the appID is in the allowed appIDs.
	if stringInSlice(appID, allowedApps) {
		return nil
	}

	// Root folder belongs to all apps.
	if fileID == "" {
		return nil
	}

	// Get the file's metadata.
	file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		return ctx.AbortWithError(http.StatusForbidden, err)
	}
	if file.GetAppID() != appID {
		return ctx.AbortWithError(http.StatusForbidden, fmt.Errorf("application not permitted"))
	}

	return nil
}

// stringInSlice checks if a given string is in a given slice of strings
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
