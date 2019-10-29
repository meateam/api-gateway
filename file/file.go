package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/upload-service/proto"
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

	// GetFileByIDRole is the role that is required of the authenticated requester to have to be
	// permitted to make the GetFileByID action.
	GetFileByIDRole = ppb.Role_READ

	// GetFilesByFolderRole is the role that is required of the authenticated requester to have to be
	// permitted to make the GetFilesByFolder action.
	GetFilesByFolderRole = ppb.Role_READ

	// DeleteFileByIDRole is the role that is required of the authenticated requester to have to be
	// permitted to make the DeleteFileByID action.
	DeleteFileByIDRole = ppb.Role_OWNER

	// DownloadRole is the role that is required of the authenticated requester to have to be
	// permitted to make the Download action.
	DownloadRole = ppb.Role_READ

	// UpdateFileRole is the role that is required of the authenticated requester to have to be
	// permitted to make the UpdateFile action.
	UpdateFileRole = ppb.Role_OWNER

	// UpdateFilesRole is the role that is required of the authenticated requester to have to be
	// permitted to make the UpdateFiles action.
	UpdateFilesRole = ppb.Role_OWNER
)

// Router is a structure that handles upload requests.
type Router struct {
	downloadClient   dpb.DownloadClient
	fileClient       fpb.FileServiceClient
	uploadClient     upb.UploadClient
	permissionClient ppb.PermissionClient
	logger           *logrus.Logger
}

// getFileByIDResponse is a structure used for parsing fpb.File to a json file metadata response.
type getFileByIDResponse struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Size        int64  `json:"size"`
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
	permissionConn *grpc.ClientConn,
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

	return r
}

// Setup sets up r and intializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET("/files", r.GetFilesByFolder)
	rg.GET("/files/:id", r.GetFileByID)
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

	if !r.HandleUserFilePermission(c, fileID, GetFileByIDRole) {
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

	c.JSON(http.StatusOK, createGetFileResponse(file))
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
	if !r.HandleUserFilePermission(c, filesParent, GetFilesByFolderRole) {
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
		responseFiles = append(responseFiles, createGetFileResponse(file))
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
		&ppb.GetUserPermissionsRequest{UserID: reqUser.ID, IsOwner: false},
	)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	files := make([]*getFileByIDResponse, 0, len(permissions.GetPermissions()))
	for _, permission := range permissions.GetPermissions() {
		file, err := r.fileClient.GetFileByID(c.Request.Context(),
			&fpb.GetByFileByIDRequest{Id: permission.GetFileID()})
		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

			return
		}

		files = append(files, createGetFileResponse(file))
	}

	c.JSON(http.StatusOK, files)
}

// DeleteFileByID is the request handler for DELETE /files/:id request.
func (r *Router) DeleteFileByID(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.String(http.StatusBadRequest, "file id is required")
		return
	}

	if !r.HandleUserFilePermission(c, fileID, DeleteFileByIDRole) {
		return
	}
	ids, err := DeleteFile(c.Request.Context(), r.logger, r.fileClient, r.uploadClient, fileID)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	if _, err = r.permissionClient.DeleteFilePermissions(
		c.Request.Context(),
		&ppb.DeleteFilePermissionsRequest{FileID: fileID}); err != nil {
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

	if !r.HandleUserFilePermission(c, fileID, DownloadRole) {
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

	if !r.HandleUserFilePermission(c, fileID, UpdateFileRole) {
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
		if !r.HandleUserFilePermission(c, *pf.Parent, UpdateFileRole) {
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
		if !r.HandleUserFilePermission(c, *body.PartialFile.Parent, UpdateFilesRole) {
			return
		}
	}

	allowedIds := make([]string, 0, len(body.IDList))

	for _, id := range body.IDList {
		isUserAllowed, err := CheckUserFilePermission(c.Request.Context(),
			r.fileClient,
			r.permissionClient,
			reqUser.ID,
			id,
			UpdateFilesRole)
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

// CheckUserFilePermission checks if userID is permitted to fileID with the wanted role.
// The function returns true if the user is permitted to the file and nil error,
// otherwise false and non-nil err if any encountered.
func CheckUserFilePermission(ctx context.Context,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	userID string,
	fileID string,
	role ppb.Role) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("userID is required")
	}

	// Everyone is permitted to their root, since all actions on root are authenticated,
	// and it's impossible to create a permission for root (aka sharing a user's whole drive).
	if fileID == "" {
		return true, nil
	}

	// Go up the hirarchy searching for a permission for userID to fileID with role.
	// Fetch fileID's parents, each at a time, and check permission to each parent.
	// If reached a parent that userID isn't permitted to then return with error,
	// If reached a parent that userID is permitted to then return true with nil error.
	// If any error encountered then return false and the encountered error.
	currentFile := fileID
	for {
		if currentFile == "" {
			return false, nil
		}

		file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: currentFile})
		if err != nil {
			return false, err
		}

		isPermitted, err := permissionClient.IsPermitted(ctx,
			&ppb.IsPermittedRequest{FileID: currentFile, UserID: userID, Role: role})
		if err != nil && status.Code(err) != codes.Unimplemented {
			return false, err
		}

		if !isPermitted.GetPermitted() && err == nil {
			return false, nil
		}

		if isPermitted.GetPermitted() {
			return true, nil
		}

		currentFile = file.GetParent()
	}
}

// CreatePermission creates permission in permission service only if userID has
// ppb.Role_OWNER permission to permission.FileID.
func CreatePermission(ctx context.Context,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	userID string,
	permission ppb.PermissionObject) error {
	// If the permission we want to create is ppb.Role_OWNER then check that there's
	// no other user that has owner permission to permission.FileID.
	if permission.GetRole() == ppb.Role_OWNER {
		filePermissions, err := permissionClient.GetFilePermissions(ctx,
			&ppb.GetFilePermissionsRequest{FileID: permission.GetFileID()})
		if err != nil {
			return fmt.Errorf("failed creating permission: %v", err)
		}

		// If there's a user with role ppb.Role_OWNER to permission.FileID
		// then we can't create another owner permission to the permission.FileID.
		for _, userPermission := range filePermissions.GetPermissions() {
			if userPermission.GetRole() == ppb.Role_OWNER {
				return fmt.Errorf("failed creating permission: there's already an owner for file %s",
					permission.GetFileID())
			}
		}
	} else {
		// Check if userID has ppb.Role_OWNER permission to permission.FileID.
		isPermitted, err := CheckUserFilePermission(ctx,
			fileClient,
			permissionClient,
			userID,
			permission.GetFileID(),
			ppb.Role_OWNER)
		if err != nil {
			return fmt.Errorf("failed creating permission: %v", err)
		}

		if !isPermitted {
			return fmt.Errorf("failed creating permission: %s is not the owner of %s",
				userID, permission.GetFileID())
		}

		if permission.GetRole() == ppb.Role_NONE && permission.GetUserID() == userID {
			return fmt.Errorf("failed creating permission: cannot remove the permission of the file owner")
		}
	}

	createPermissionRequest := ppb.CreatePermissionRequest{
		FileID: permission.GetFileID(),
		UserID: permission.GetUserID(),
		Role:   permission.GetRole(),
	}
	_, err := permissionClient.CreatePermission(ctx, &createPermissionRequest)
	if err != nil {
		return fmt.Errorf("failed creating permission: %v", err)
	}

	return nil
}

// HandleUserFilePermission gets a gin context and the id of the requested file,
// returns true if the user is permitted to operate on the file.
// Returns false if the user isn't permitted to operate on it,
// Returns false if any error occurred and logs the error.
func (r *Router) HandleUserFilePermission(c *gin.Context, fileID string, role ppb.Role) bool {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return false
	}

	isPermitted, err := CheckUserFilePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		fileID,
		role)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return false
	}

	return isPermitted
}

// createGetFileResponse Creates a file grpc response to http response struct
func createGetFileResponse(file *fpb.File) *getFileByIDResponse {
	if file == nil {
		return nil
	}

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

	return responseFile
}
