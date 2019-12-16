package file

import (
	"context"
	"fmt"
	"sync"

	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	spb "github.com/meateam/search-service/proto"
	upb "github.com/meateam/upload-service/proto"
	"github.com/sirupsen/logrus"
)

// DeleteFile deletes fileID from file service and upload service, returns a slice of IDs of the files
// that were deleted if there were any files that are descendants of fileID and any error if occurred.
func DeleteFile(ctx context.Context,
	logger *logrus.Logger,
	fileClient fpb.FileServiceClient,
	uploadClient upb.UploadClient,
	searchClient spb.SearchClient,
	fileID string) ([]string, error) {
	// IMPORTANT TODO: need to check permissions per file that descends from fileID.
	deleteFileRequest := &fpb.DeleteFileRequest{
		Id: fileID,
	}
	deleteFileResponse, err := fileClient.DeleteFile(ctx, deleteFileRequest)
	if err != nil {
		return nil, err
	}

	bucketKeysMap := make(map[string][]string)
	ids := make([]string, len(deleteFileResponse.GetFiles()))
	for i, file := range deleteFileResponse.GetFiles() {
		bucketKeysMap[file.GetBucket()] = append(bucketKeysMap[file.GetBucket()], file.GetKey())
		ids[i] = file.GetId()

		if _, err := searchClient.Delete(ctx, &spb.DeleteRequest{Id: file.GetId()}); err != nil {
			loggermiddleware.LogError(logger, err)
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

	if len(ids) <= 0 {
		return nil, fmt.Errorf("failed deleting files")
	}

	return ids, nil
}
