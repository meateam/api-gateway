package upload

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"

	qpb "github.com/meateam/file-service/proto/quota"
	upb "github.com/meateam/upload-service/proto"
	usb "github.com/meateam/user-service/proto/users"
)

const (
	// FileIDBody -file id body paramater name
	FileIDBody = "fileId"

	// DestOwnerBody - destination owner paramater name
	DestOwnerBody = "newOwnerId"

	// UploadBucketCustomHeader ..
	UploadBucketCustomHeader = "uploadBucket"

	// UploadKeyCustomHeader ..
	UploadKeyCustomHeader = "uploadKey"

	// ShiftQueryKey the query string key name for shift type
	ShiftQueryKey = "type"

	// 	ShiftCopyType - the value for copy type
	ShiftCopyType = "copy"

	// ShiftMoveType - the value for move type
	ShiftMoveType = "move"

	// CopyFileRole is the role that is required of the authenticated requester to have to be
	// permitted to make the copy file action.
	CopyFileRole = ppb.Role_READ

	// LargeFileSize is 5GB
	LargeFileSize = 5 << 30
)

type ShiftInit struct {
	FileID     string `json:"fileId"`
	NewOwnerID string `json:"newOwnerId"`
	ReqUserID  string `json:"reqUserId"`
}

// ShiftFileBody is a structure of the json body of shift request.
type ShiftFileBody struct {
	FileID     string `json:"fileId"`
	NewOwnerID string `json:"newOwnerId"`
}

// copyFileRequest is a structure of the json body for transfer between buckets
type copyFileRequest struct {
	file       *fpb.File
	newOwnerID string
	parentID   *string
}

type FileCopy struct {
	file   *fpb.File
	result string
}

// ShiftSetup initializes its routes under rg.
func (r *Router) ShiftSetup(rg *gin.RouterGroup) {
	rg.POST("/shift", r.Shift)
}

// Shift is the request handler for /shift request.
// shift objects between buckets
func (r *Router) Shift(c *gin.Context) {
	// Get the user from request
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Get the shift type query string and check if it's valid
	shiftType, exists := c.GetQuery(ShiftQueryKey)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("shift %s query is required", ShiftQueryKey))
		return
	}
	if shiftType != ShiftCopyType && shiftType != ShiftMoveType {
		c.String(http.StatusBadRequest, fmt.Sprintf("shiftType: %s doesnt supported", shiftType))
		return
	}

	// Get the shift file request body and check if it's valid
	var reqBody ShiftFileBody
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unexpected body format")),
		)

		return
	}

	if reqBody.FileID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", FileIDBody))
		return
	}

	if reqBody.NewOwnerID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", DestOwnerBody))
		return
	}

	// Check if the dest owner id is exists
	destUser, err := r.userClient().GetUserByID(c.Request.Context(), &usb.GetByIDRequest{Id: reqBody.NewOwnerID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	// Check user validation - if the user is not valid, the request will be aborted
	if destUser.GetUser() == nil || destUser.GetUser().GetId() != reqBody.NewOwnerID {
		c.String(http.StatusBadRequest, "problem with dest user")
		return
	}

	if destUser.GetUser().GetId() == reqUser.ID {
		c.String(http.StatusBadRequest, "cant do move/copy opreation to yourself")
		return
	}

	r.shiftInit(c, ShiftInit{FileID: reqBody.FileID, NewOwnerID: reqBody.NewOwnerID, ReqUserID: reqUser.ID})
}

// shiftInit - initialize copy or move request
func (r *Router) shiftInit(c *gin.Context, Shift ShiftInit) {
	r.logger.Info("shiftInit")

	// Get the file by id
	file, err := r.fileClient().GetFileByID(c.Request.Context(), &fpb.GetByFileByIDRequest{Id: Shift.FileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}
	r.logger.Info("shiftInit GetFileByID")

	// Check if the user has the required role to perform the action (move or copy)
	hasRequiredRole := r.hasRequiredRole(c, file, Shift.ReqUserID)
	if !hasRequiredRole {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("You do not have permission to do this operation")))
		return
	}
	r.logger.Info("shiftInit hasRequiredRole")

	// Check if dest user remaining quota is less than the file size
	getFileSizeReq := &fpb.GetFileSizeByIDRequest{Id: Shift.FileID, OwnerID: Shift.NewOwnerID}
	getFileSizeRes, err := r.fileClient().GetFileSizeByID(c.Request.Context(), getFileSizeReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}
	r.logger.Info("shiftInit GetFileSizeByID")

	r.hasAvailableQuota(c, getFileSizeRes.GetFileSize(), Shift.NewOwnerID)
	r.logger.Info("shiftInit hasAvailableQuota")

	// Copy item to the parent folder
	// TODO: check if we need to change default location
	var newParentID *string = nil

	// Create file upload in db and allocate quota - with the new owner and parent of the file
	uploadObjectReq := &fpb.CreateUploadRequest{
		Bucket:  Shift.NewOwnerID,
		Name:    file.GetName(),
		OwnerID: Shift.NewOwnerID,
		Parent:  *newParentID,
		Size:    getFileSizeRes.GetFileSize(),
	}
	upload, err := r.fileClient().CreateUpload(c.Request.Context(), uploadObjectReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}
	r.logger.Info("shiftInit CreateUpload")

	// Copy the file and it's desendents to the new owner
	copyReq := copyFileRequest{file: file, newOwnerID: Shift.NewOwnerID, parentID: newParentID}
	if err := r.copyObjects(c, copyReq); err != nil {
		loggermiddleware.LogError(r.logger, err)
	}

	// Remove the upload and release the upload quota
	if _, errdelete := r.fileClient().DeleteUploadByKey(c.Request.Context(), &fpb.DeleteUploadByKeyRequest{Key: upload.GetKey()}); errdelete != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}
	r.logger.Info("shiftInit DeleteUploadByKey")

	c.Header(UploadBucketCustomHeader, upload.GetBucket())
	c.Header(UploadKeyCustomHeader, upload.GetKey())
	c.Status(http.StatusOK)
}

// copyObjects - copy a file or a folder and it's descendsents to the new owner
func (r *Router) copyObjects(c *gin.Context, copyObjectRequest copyFileRequest) error {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		return c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}

	fileID := copyObjectRequest.file.GetId()

	// Get descendants by file id - if it's a file (and not a folder), it will return an empty array
	descendantsResp, err := r.fileClient().GetDescendantsByID(c.Request.Context(), &fpb.GetDescendantsByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return c.AbortWithError(httpStatusCode, err)
	}

	// Create files array to copy to the new owner
	files := make([]*fpb.File, 0, len(descendantsResp.GetDescendants())+1)
	files = append(files, copyObjectRequest.file)

	descendants := descendantsResp.GetDescendants()
	for _, descendant := range descendants {
		files = append(files, descendant.GetFile())
	}

	// Copy each descendant in the folder whose owner is the user that made the request.
	failedCopyStorageFiles, successCopyStorageFiles := r.copyObjectManipulate(c, files, copyObjectRequest.newOwnerID)
	if len(failedCopyStorageFiles) > 0 {
		r.copyObjectToBucketRollack(c, successCopyStorageFiles, copyObjectRequest.newOwnerID)
		return c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Error while copy objects between buckets"))
	}

	isMoving := isMoveShift(c)

	// If move
	if isMoving {
		// Change owner for each descendant
		failedChangeOwnerFiles, successChangeOwnerFiles := r.changeOwnerMoveManipulate(c, successCopyStorageFiles, copyObjectRequest)
		if len(failedChangeOwnerFiles) > 0 {
			// Rollback for owners
			r.changeOwnerRollbackMove(c, successChangeOwnerFiles, reqUser.ID)

			// Rollback for buckets
			r.copyObjectToBucketRollack(c, successCopyStorageFiles, copyObjectRequest.newOwnerID)

			return c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Error while change owner"))

		}

		// Change owner to the root
		if err := changeOwnerMove(r, c, copyObjectRequest, successCopyStorageFiles[fileID].result); err != nil {
			// Rollback for owners
			r.changeOwnerRollbackMove(c, successChangeOwnerFiles, reqUser.ID)

			// Rollback for buckets
			r.copyObjectToBucketRollack(c, successCopyStorageFiles, copyObjectRequest.newOwnerID)

			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			return c.AbortWithError(httpStatusCode, err)
		}

		filesKeys := make([]string, 0, len(descendantsResp.GetDescendants())+1)
		filesIds := make([]string, 0, len(descendantsResp.GetDescendants())+1)

		for _, file := range files {
			filesKeys = append(filesKeys, file.GetKey())
			filesIds = append(filesIds, file.GetId())
		}

		// Remove files from db

		// Remove files from the source bucket
		deleteReq := &upb.DeleteObjectsRequest{Bucket: reqUser.Bucket, Keys: filesKeys}
		if _, err := r.uploadClient().DeleteObjects(c.Request.Context(), deleteReq); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			return c.AbortWithError(httpStatusCode, err)
		}

		return nil
	} else {
		// Change owner for each descendant
		failedChangeOwnerFiles, _, err := r.changeOwnerCopyManipulate(c, successCopyStorageFiles, copyObjectRequest)
		if err != nil || len(failedChangeOwnerFiles) > 0 {
			// Rollback for owners
			r.changeOwnerRollbackCopy(c, fileID)

			// Rollback for buckets
			r.copyObjectToBucketRollack(c, successCopyStorageFiles, copyObjectRequest.newOwnerID)

			return c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Error while change owner"))

		}

		if err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			return c.AbortWithError(httpStatusCode, err)
		}
	}
	return nil
}

// copyObjectManipulate -function that manipulates copy object. get files array and manipulate them.
func (r *Router) copyObjectManipulate(
	c *gin.Context,
	files []*fpb.File,
	newOwnerID string) ([]*fpb.File, map[string]*FileCopy) {
	wg := &sync.WaitGroup{}
	mu := sync.Mutex{}

	// Create slices to send the results
	copyFailed := make([]*fpb.File, 0, len(files))
	copySuccessful := make(map[string]*FileCopy)

	defer wg.Wait()
	for _, file := range files {
		wg.Add(1)

		go func(file *fpb.File) {
			parent := file.GetParent()

			copyFile := copyFileRequest{
				file:       file,
				newOwnerID: newOwnerID,
				parentID:   &parent,
			}

			if file.GetType() != FolderContentType {
				destKey, err := copyObjectToBucket(r, c, copyFile)
				if err != nil {
					mu.Lock()
					copyFailed = append(copyFailed, file)
					mu.Unlock()

					return
				}

				mu.Lock()
				copySuccessful[file.GetId()] = &FileCopy{file: file, result: destKey}
				mu.Unlock()

			}

			wg.Done()
		}(file)

	}

	return copyFailed, copySuccessful
}

// copyObjectToBucket - copy an object between buckets and change owner
// retruns the new key of the object if the copy operation was succesful
func copyObjectToBucket(r *Router, c *gin.Context, copyFileRequest copyFileRequest) (string, error) {
	file := copyFileRequest.file

	// If the file size is larger than 5gb, we can't use the copy function
	// and we need to do a multipart upload and delete
	// s3 doesn't allowed to copy objects larger than 5gb between buckets
	if file.GetSize() > LargeFileSize {
		CopyLargeFile(c, file)
		return "", nil
	}

	// Generate a new destination key
	keyResp, err := r.fileClient().GenerateKey(c.Request.Context(), &fpb.GenerateKeyRequest{})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return "", c.AbortWithError(httpStatusCode, err)
	}

	keySrc := file.GetKey()
	keyDest := keyResp.GetKey()

	// Copy object between buckets
	copyObjectReq := &upb.CopyObjectRequest{
		BucketSrc:  file.GetBucket(),
		BucketDest: user.NormalizeCephBucketName(copyFileRequest.newOwnerID),
		KeySrc:     keySrc,
		KeyDest:    keyDest,
	}

	// Copy the objects between buckets
	if _, err = r.uploadClient().CopyObject(c.Request.Context(), copyObjectReq); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return "", c.AbortWithError(httpStatusCode, err)
	}

	return keyDest, nil
}

// CopyBucketRollBack - function that rollback the bucket changes for the files
func (r *Router) copyObjectToBucketRollack(c *gin.Context, successCopyStorageFiles map[string]*FileCopy, newOwnerID string) error {
	keys := make([]string, 0, len(successCopyStorageFiles))
	for _, successCopyStorageFile := range successCopyStorageFiles {
		keys = append(keys, successCopyStorageFile.result)
	}

	// Delete the objects from the destination bucket
	deleteReq := &upb.DeleteObjectsRequest{Bucket: user.NormalizeCephBucketName(newOwnerID), Keys: keys}

	if _, err := r.uploadClient().DeleteObjects(c.Request.Context(), deleteReq); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return c.AbortWithError(httpStatusCode, err)
	}

	return nil
}

// changeOwnerManipulate -function that manipulates copy object. get files array and manipulate them.
func (r *Router) changeOwnerMoveManipulate(
	c *gin.Context,
	files map[string]*FileCopy,
	copyObjectReq copyFileRequest) ([]*fpb.File, []*fpb.File) {
	wg := &sync.WaitGroup{}
	mu := sync.Mutex{}

	// Create slices to send the results
	copyFailed := make([]*fpb.File, 0, len(files))
	copySuccessful := make([]*fpb.File, 0, len(files))

	defer wg.Wait()
	for _, fileCopy := range files {
		wg.Add(1)

		go func(fileCopy *FileCopy) {
			parent := fileCopy.file.GetParent()

			copyFile := copyFileRequest{
				file:       fileCopy.file,
				newOwnerID: copyObjectReq.newOwnerID,
				parentID:   &parent,
			}

			err := changeOwnerMove(r, c, copyFile, fileCopy.result)
			if err != nil {
				mu.Lock()
				copyFailed = append(copyFailed, fileCopy.file)
				mu.Unlock()

				return
			}

			mu.Lock()
			copySuccessful = append(copySuccessful, fileCopy.file)
			mu.Unlock()

			wg.Done()
		}(fileCopy)

	}

	copyFile := copyFileRequest{
		file:       copyObjectReq.file,
		newOwnerID: copyObjectReq.newOwnerID,
		parentID:   copyObjectReq.parentID,
	}

	err := changeOwnerMove(r, c, copyFile, files[copyObjectReq.file.Id].result)
	if err != nil {
		copyFailed = append(copyFailed, copyObjectReq.file)
	}
	copySuccessful = append(copySuccessful, copyObjectReq.file)

	return copyFailed, copySuccessful
}

// ChangeOwner ...
func changeOwnerMove(r *Router, c *gin.Context, copyFileRequest copyFileRequest, destKey string) error {
	updateFileRequest := &fpb.UpdateFilesRequest{
		IdList: []string{copyFileRequest.file.GetId()},
		PartialFile: &fpb.File{
			Bucket:   user.NormalizeCephBucketName(copyFileRequest.newOwnerID),
			Key:      destKey,
			OwnerID:  copyFileRequest.newOwnerID,
			FileOrId: &fpb.File_Parent{*copyFileRequest.parentID},
		},
	}

	// Update the file in db
	updateFilesResponse, err := r.fileClient().UpdateFiles(c.Request.Context(), updateFileRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return c.AbortWithError(httpStatusCode, err)
	}

	// Only refers to one, because it cannot update more than one
	if len(updateFilesResponse.GetFailedFiles()) != 0 {
		failedFileID := updateFilesResponse.GetFailedFiles()[0]
		return c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Error while updating file %s", failedFileID))
	}

	return nil
}

// changeOwnerCopyManipulate -
func (r *Router) changeOwnerCopyManipulate(c *gin.Context, files map[string]*FileCopy, copyObjectReq copyFileRequest) ([]*fpb.File, []*FileCopy, error) {
	// Create slices to send the results
	copyFailed := make([]*fpb.File, 0, len(files))
	copySuccessful := make([]*FileCopy, 0, len(files))

	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		return copyFailed, copySuccessful, c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}

	fileID, err := changeOwnerCopy(r, c, copyObjectReq, files[copyObjectReq.file.Id].result)
	if err != nil {
		copyFailed = append(copyFailed, copyObjectReq.file)

		return copyFailed, copySuccessful, nil
	}
	copySuccessful = append(copySuccessful, &FileCopy{file: copyObjectReq.file, result: fileID})

	getDescendantsByFolderReq := fpb.GetDescendantsByFolderRequest{FolderID: fileID, OwnerID: reqUser.ID}
	descendants, err := r.fileClient().GetDescendantsByFolder(c.Request.Context(), &getDescendantsByFolderReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return copyFailed, copySuccessful, c.AbortWithError(httpStatusCode, err)
	}

	var copyDescendants func(parentID string, descendant *fpb.File)
	copyDescendants = func(parentID string, descendant *fpb.File) {
		copyFile := copyFileRequest{
			file:       descendant,
			newOwnerID: copyObjectReq.newOwnerID,
			parentID:   &parentID,
		}

		newFileID, err := changeOwnerCopy(r, c, copyFile, files[descendant.GetId()].result)
		if err != nil {
			copyFailed = append(copyFailed, copyObjectReq.file)

			return
		}

		copySuccessful = append(copySuccessful, &FileCopy{file: descendant, result: newFileID})

		for _, descendant := range descendant.GetChildren() {
			copyDescendants(newFileID, descendant)
		}

	}

	for _, descendant := range descendants.GetFiles() {
		copyDescendants(fileID, descendant)
	}

	return copyFailed, copySuccessful, nil
}

// ChangeOwner ...
func changeOwnerCopy(r *Router, c *gin.Context, copyFileRequest copyFileRequest, destKey string) (string, error) {
	// Create new file instance for the same file
	file := copyFileRequest.file

	createFileReq := &fpb.CreateFileRequest{
		Key:     destKey,
		Bucket:  user.NormalizeCephBucketName(copyFileRequest.newOwnerID),
		OwnerID: copyFileRequest.newOwnerID,
		Size:    file.GetSize(),
		Type:    file.GetType(),
		Name:    file.GetName(),
		Parent:  *copyFileRequest.parentID,
		AppID:   file.GetAppID(),
	}

	// Create the new file in db
	createFileResponse, err := r.fileClient().CreateFile(c.Request.Context(), createFileReq)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		return "", c.AbortWithError(httpStatusCode, err)
	}

	return createFileResponse.GetId(), nil
}

// ChangeOwnerRollBack - function that rollback the owner changes for the files
func (r *Router) changeOwnerRollbackMove(c *gin.Context, successChangeOwnerFiles []*fpb.File, reqUserID string) error {
	var wg sync.WaitGroup

	defer wg.Wait()
	// Rollback for change owner
	for _, successChangeOwnerFile := range successChangeOwnerFiles {
		wg.Add(1)

		go func(successChangeOwnerFile *fpb.File) {
			copyFile := copyFileRequest{
				file:       successChangeOwnerFile,
				newOwnerID: reqUserID,
			}

			changeOwnerMove(r, c, copyFile, successChangeOwnerFile.GetKey())

			wg.Done()
		}(successChangeOwnerFile)
	}

	return nil
}

// ChangeOwnerRollBack - function that rollback the owner changes for the files
func (r *Router) changeOwnerRollbackCopy(c *gin.Context, fileID string) error {
	// Delete the file from db and it's descedants
	deleteFileReq := &fpb.DeleteFileRequest{Id: fileID}

	if _, err := r.fileClient().DeleteFile(c.Request.Context(), deleteFileReq); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		c.AbortWithError(httpStatusCode, err)
	}

	return nil
}

// CopyLargeFile ...
func CopyLargeFile(c *gin.Context, file *fpb.File) {
	// TODO: implement copy CopyLargeFile
	// calls multipart upload file and delete
}

// hasRequiredRole - returns true if the user has the required role
// (owner for move, read priviliges for copy)
func (r *Router) hasRequiredRole(c *gin.Context, file *fpb.File, userID string) bool {
	// Copy case - requires read priviliges
	if !isMoveShift(c) {
		if hasPermission := r.HandleUserFilePermission(c, file.GetId(), CopyFileRole); !hasPermission {
			return false
		}
	} else {
		// Check if the owner of the current file isn't the requesting user,
		// if so then he's not permitted to change the ownership
		if file.GetOwnerID() != userID {
			return false
		}
	}

	return true
}

// hasAvailableQuota - check if the dest user has enough quota for the copied objects
func (r *Router) hasAvailableQuota(c *gin.Context, fileSize int64, userID string) {
	// Check if dest user remaining quota is less than the file size
	quota, err := r.quotaClient().GetOwnerQuota(c.Request.Context(), &qpb.GetOwnerQuotaRequest{OwnerID: userID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	remainQuota := quota.GetLimit() - quota.GetUsed() - fileSize
	if remainQuota < 0 {
		c.String(http.StatusForbidden, "Not enough quota")
		return
	}
}

// isMoveShift - returns true if the user requested to move the file.
// return false if the user requested to copy the file.
func isMoveShift(c *gin.Context) bool {
	return c.Query(ShiftQueryKey) == ShiftMoveType
}
