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
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	dlgpb "github.com/meateam/delegation-service/proto/delegation-service"
	"github.com/meateam/download-service/download"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	"github.com/meateam/gotenberg-go-client/v6"
	ppb "github.com/meateam/permission-service/proto"
	ptpb "github.com/meateam/permit-service/proto"
	spb "github.com/meateam/search-service/proto"
	upb "github.com/meateam/upload-service/proto"
	usrpb "github.com/meateam/user-service/proto/users"
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

	// QueryShareFiles is the querystring key for retrieving the files that were shared with the user.
	QueryShareFiles = "shares"

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
)

// Router is a structure that handles upload requests.
type Router struct {
	downloadClient   dpb.DownloadClient
	fileClient       fpb.FileServiceClient
	uploadClient     upb.UploadClient
	permissionClient ppb.PermissionClient
	permitClient     ptpb.PermitClient
	searchClient     spb.SearchClient
	userClient       usrpb.UsersClient
	delegationClient dlgpb.DelegationClient
	gotenbergClient  *gotenberg.Client
	oAuthMiddleware  *oauth.Middleware
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
	Parent      string      `json:"parent,omitempty"`
	CreatedAt   int64       `json:"createdAt,omitempty"`
	UpdatedAt   int64       `json:"updatedAt,omitempty"`
	Role        string      `json:"role,omitempty"`
	Shared      bool        `json:"shared"`
	Permission  *Permission `json:"permission,omitempty"`
	IsExternal  bool        `json:"isExternal"`
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
	permitConn *grpc.ClientConn,
	searchConn *grpc.ClientConn,
	userConn *grpc.ClientConn,
	delegationConn *grpc.ClientConn,
	gotenbergClient *gotenberg.Client,
	oAuthMiddleware *oauth.Middleware,
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
	r.permitClient = ptpb.NewPermitClient(permitConn)
	r.searchClient = spb.NewSearchClient(searchConn)
	r.userClient = usrpb.NewUsersClient(userConn)
	r.delegationClient = dlgpb.NewDelegationClient(delegationConn)
	r.gotenbergClient = gotenbergClient

	r.oAuthMiddleware = oAuthMiddleware

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	checkExternalAdminScope := r.oAuthMiddleware.ScopeMiddleware(oauth.OutAdminScope)

	rg.GET("/files", checkExternalAdminScope, r.GetFilesByFolder)
	rg.GET("/files/:id", checkExternalAdminScope, r.GetFileByID)
	rg.GET("/files/:id/ancestors", r.GetFileAncestors)
	rg.DELETE("/files/:id", r.DeleteFileByID)
	rg.PUT("/files/:id", r.UpdateFile)
	rg.PUT("/files", r.UpdateFiles)
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
		if !r.HandleUserFilePermit(c, fileID, GetFileByIDRole) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
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
	role, _ := r.HandleUserFilePermission(c, fileID, GetFileByIDRole)
	if role == "" {
		if !r.HandleUserFilePermit(c, fileID, DownloadRole) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
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

// CheckUserFilePermit checks if userID is has a permit to fileID.
// The function returns true if the user has a permit to the file and nil error,
// otherwise false and non-nil err if any encountered.
func CheckUserFilePermit(ctx context.Context,
	permitClient ptpb.PermitClient,
	userID string,
	fileID string,
	role ppb.Role) (bool, error) {

	// Permits have only READ roles
	if role != ppb.Role_READ {
		return false, nil
	}

	hasPermitRes, err := permitClient.HasPermit(ctx, &ptpb.HasPermitRequest{FileID: fileID, UserID: userID})
	if err != nil {
		return false, err
	}

	hasPermit := hasPermitRes.GetHasPermit()

	return hasPermit, nil
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

	isPermitted, err := CheckUserFilePermit(c.Request.Context(),
		r.permitClient,
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
