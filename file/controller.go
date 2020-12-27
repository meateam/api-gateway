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
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
)

// func deleteFileAndPrem(ctx *gin.Context,
// 	logger *logrus.Logger,
// 	fileClient fpb.FileServiceClient,
// 	permissionClient ppb.PermissionClient,
// 	fileID string) {
// 	// Delete permissions
// 	permissionsFile, err := permissionClient.GetFilePermissions(ctx, &ppb.GetFilePermissionsRequest{FileID: fileID})
// 	if err != nil {
// 		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
// 		loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, fmt.Errorf("failed to get file permission to delete: %v", err)))
// 	}

// 	if _, err := permissionClient.DeleteFilePermissions(ctx, &ppb.DeleteFilePermissionsRequest{FileID: fileID}); err != nil {
// 		loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file %s permissions: %v", fileID, err))
// 	}

// 	// Delete file from db
// 	deletedFile, err := fileClient.DeleteFileByID(ctx, &fpb.DeleteFileByIDRequest{Id: file.GetId()})
// 	if err != nil {
// 		// TODO: add permission rollback
// 		loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file: %v", err))
// 	}

// 	if deletedFile == nil {
// 		// TODO: add permission rollback
// 		loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file %s", file.GetId()))
// 	} else {
// 		return deletedFile.GetFile()
// 	}
// }

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
	userID string) ([]string, error) {
	file, err := fileClient.GetFileByID(ctx, &fpb.GetByFileByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, fmt.Errorf("failed getting file to delete: %v", err)))
	}

	res, err := fileClient.GetDescendantsByID(ctx, &fpb.GetDescendantsByIDRequest{Id: fileID})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(logger, ctx.AbortWithError(httpStatusCode, fmt.Errorf("failed getting file's descendants to delete: %v", err)))
	}

	descendants := res.GetDescendants()
	deletedFiles := make([]*fpb.File, 0, len(descendants)+1)
	floatFiles := make([]string, 0, len(descendants))

	// Deleting the file from db
	// TODO: change that the permission delete first before file
	// Only the owner of the file can delete the file instance.
	// If the user requesting to delete isn't the owner- delete it's permission to this file
	if file.GetOwnerID() == userID {

		// TODO: change to function delete file
		deletedFile, err := fileClient.DeleteFileByID(ctx, &fpb.DeleteFileByIDRequest{Id: file.GetId()})
		if err != nil {
			// TODO: add permission rollback
			loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file: %v", err))
		}

		if deletedFile == nil {
			// TODO: add permission rollback
			loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file %s", file.GetId()))
		} else {
			deletedFiles = append(deletedFiles, deletedFile.GetFile())
		}
	} else {
		if _, err := permissionClient.DeletePermission(
			ctx,
			&ppb.DeletePermissionRequest{FileID: fileID, UserID: userID}); err != nil {
			return nil, fmt.Errorf("failed deleting user %s permission to file %s: %v", userID, fileID, err)
		}
	}

	// Delete file's descendants
	for i := 0; i < len(descendants); i++ {
		file := descendants[i].GetFile()
		parent := descendants[i].GetParent()

		if file.GetOwnerID() == userID {
			// TODO: change to function delete file
			deletedFile, err := fileClient.DeleteFileByID(ctx, &fpb.DeleteFileByIDRequest{Id: file.GetId()})
			if err != nil {
				loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file: %v", err))
				// TODO: add permission rollback
			}

			if deletedFile == nil {
				loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file %s", file.GetId()))
				// TODO: add permission rollback

			} else {
				deletedFiles = append(deletedFiles, deletedFile.GetFile())
			}
		} else if parent == nil || parent.GetOwnerID() == userID {
			// TODO: ask what it is??
			floatFiles = append(floatFiles, file.GetId())
		}
	}

	// TODO: ask what it is??
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
		bucketKeysMap[file.GetBucket()] = append(bucketKeysMap[file.GetBucket()], file.GetKey())
		ids = append(ids, file.GetId())

		if _, err := searchClient.Delete(ctx, &spb.DeleteRequest{Id: file.GetId()}); err != nil {
			loggermiddleware.LogError(logger, err)
		}

		// TODO: change this to do first
		_, err := permissionClient.DeleteFilePermissions(ctx, &ppb.DeleteFilePermissionsRequest{FileID: file.GetId()})
		if err != nil {
			loggermiddleware.LogError(logger, fmt.Errorf("failed deleting file %s permissions: %v", file.GetId(), err))
		}
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	for bucket, keys := range bucketKeysMap {
		wg.Add(1)
		go func(bucket string, keys []string) {
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

			wg.Done()
		}(bucket, keys)
	}

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
