package file

import (
	"context"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	spb "github.com/meateam/search-service/proto"
	upb "github.com/meateam/upload-service/proto"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// deleteFileAndPremission deletes the file and the permissions to it from db
func deleteFileAndPremission(ctx *gin.Context,
	logger *logrus.Logger,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	fileID string) *fpb.File {
	// Delete file's permissions
	deletedPermissions, err := permissionClient.DeleteFilePermissions(ctx, &ppb.DeleteFilePermissionsRequest{FileID: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, fmt.Errorf("failed deleting file's %s permissions: %v", fileID, err)))
		return nil
	}

	deletedFilePermissions := deletedPermissions.GetPermissions()

	filePermissions := make([]*ppb.GetFilePermissionsResponse_UserRole, 0, len(deletedFilePermissions))
	for _, deletedFilePermission := range deletedFilePermissions {
		filePermissions = append(filePermissions, &ppb.GetFilePermissionsResponse_UserRole{
			UserID:  deletedFilePermission.GetUserID(),
			Role:    deletedFilePermission.GetRole(),
			Creator: deletedFilePermission.GetCreator(),
		})
	}

	// Delete file from db
	deletedFile, err := fileClient.DeleteFileByID(ctx, &fpb.DeleteFileByIDRequest{Id: fileID})
	if err != nil || deletedFile == nil {
		if status.Code(err) != codes.NotFound {
			// Permission rollback
			AddPermissionsOnError(ctx, err, fileID, filePermissions, permissionClient, logger)
		}

		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, fmt.Errorf("failed deleting file: %v", err)))
		return nil
	}

	return deletedFile.GetFile()
}

// DeleteFile deletes fileID from file service and upload service, returns a slice of IDs of the files
// that were deleted if there were any files that are descendants of fileID and any error if occurred.
// nolint: gocyclo
func DeleteFile(ctx *gin.Context,
	logger *logrus.Logger,
	fileClient fpb.FileServiceClient,
	uploadClient upb.UploadClient,
	searchClient spb.SearchClient,
	permissionClient ppb.PermissionClient,
	fileID string,
	userID string,
	role string) ([]string, error) {
	swg := sizedwaitgroup.New(10)
	wg := &sync.WaitGroup{}
	mu := sync.RWMutex{}

	var deletedFile *fpb.File = nil

	// Deleting the file and permissions from db
	// Only the owner of the file can delete the file instance.
	// If the user requesting to delete isn't the owner- delete it's permission to this file
	if role == OwnerRole {
		deletedFile = deleteFileAndPremission(ctx, logger, fileClient, permissionClient, fileID)
	} else {
		if _, err := permissionClient.DeletePermission(
			ctx,
			&ppb.DeletePermissionRequest{FileID: fileID, UserID: userID}); err != nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
			errFmt := fmt.Errorf("failed getting file's descendants to delete: %v", err)
			loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, errFmt))
			return nil, errFmt
		}
	}

	getDescendantsByIDRes, err := fileClient.GetDescendantsByID(ctx, &fpb.GetDescendantsByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		errFmt := fmt.Errorf("failed getting file's descendants to delete: %v", err)
		loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, errFmt))
		return nil, errFmt
	}

	descendants := getDescendantsByIDRes.GetDescendants()
	deletedFiles := make([]*fpb.File, 0, len(descendants)+1)
	floatFiles := make([]string, 0, len(descendants))

	if deletedFile != nil {
		deletedFiles = append(deletedFiles, deletedFile)
	}

	// Delete file's descendants
	for _, descendant := range descendants {
		swg.Add()

		go func(descendant *fpb.GetDescendantsByIDResponse_Descendant, deletedFiles *[]*fpb.File, floatFiles *[]string) {
			defer swg.Done()
			file := descendant.GetFile()
			parent := descendant.GetParent()

			if file.GetOwnerID() == userID {
				deletedFile := deleteFileAndPremission(ctx, logger, fileClient, permissionClient, file.GetId())
				if deletedFile != nil {
					mu.Lock()
					*deletedFiles = append(*deletedFiles, deletedFile)
					mu.Unlock()
				}

			} else if parent == nil || parent.GetOwnerID() == userID {
				mu.Lock()
				*floatFiles = append(*floatFiles, file.GetId())
				mu.Unlock()
			}
		}(descendant, &deletedFiles, &floatFiles)
	}
	swg.Wait()
	root := ""
	failedFloatFiles, err := HandleUpdate(
		ctx,
		floatFiles,
		partialFile{Float: true, Parent: &root},
		fileClient,
		uploadClient,
		searchClient,
		logger)
	if err != nil {
		loggermiddleware.LogError(logger, fmt.Errorf("failed updating files: %v to float: %v", failedFloatFiles, err))
	}

	if len(failedFloatFiles) > 0 {
		loggermiddleware.LogError(logger, fmt.Errorf("failed updating files: %v to float", failedFloatFiles))
	}

	bucketKeysMap := make(map[string][]string)
	ids := make([]string, 0, len(deletedFiles))
	for _, file := range deletedFiles {
		swg.Add()
		go func(file *fpb.File, bucketKeysMap *map[string][]string, ids *[]string) {
			defer swg.Done()
			mu.Lock()
			(*bucketKeysMap)[file.GetBucket()] = append((*bucketKeysMap)[file.GetBucket()], file.GetKey())
			*ids = append(*ids, file.GetId())
			mu.Unlock()

			if _, err := searchClient.Delete(ctx, &spb.DeleteRequest{Id: file.GetId()}); err != nil {
				loggermiddleware.LogError(logger, err)
			}
		}(file, &bucketKeysMap, &ids)
	}
	swg.Wait()
	mu.RLock()

	for bucket, keys := range bucketKeysMap {
		wg.Add(1)
		go func(bucket string, keys []string) {
			defer wg.Done()
			DeleteObjectRequest := &upb.DeleteObjectsRequest{
				Bucket: bucket,
				Keys:   keys,
			}

			deleteObjectResponse, err := uploadClient.DeleteObjects(ctx, DeleteObjectRequest)
			if err != nil || len(deleteObjectResponse.GetFailed()) > 0 {
				loggermiddleware.LogError(logger, err)
			}
			if len(deleteObjectResponse.GetFailed()) > 0 {
				loggermiddleware.LogError(
					logger,
					fmt.Errorf("failed deleting keys: %v", deleteObjectResponse.GetFailed()),
				)
			}
		}(bucket, keys)
	}
	wg.Wait()
	mu.RUnlock()

	if len(ids) < len(deletedFiles) {
		return nil, fmt.Errorf("failed deleting files")
	}

	return ids, nil
}

// HandleUpdate updates many files with the same value.
// The function gets slice of ids and the partial file to update.
// It returns the updated file id's, and non-nil error if occurred.
func HandleUpdate(
	ctx context.Context,
	ids []string,
	pf partialFile,
	fileClient fpb.FileServiceClient,
	uploadClient upb.UploadClient,
	searchClient spb.SearchClient,
	logger *logrus.Logger) ([]*fpb.FailedFile, error) {
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

	updateFilesResponse, err := fileClient.UpdateFiles(
		ctx,
		&fpb.UpdateFilesRequest{
			IdList:      ids,
			PartialFile: updatedData,
		},
	)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		sUpdatedData.Id = id
		if _, err := searchClient.Update(ctx, sUpdatedData); err != nil {
			logger.Errorf("failed to update file %s in searchService", id)
		}
	}

	return updateFilesResponse.GetFailedFiles(), nil
}
