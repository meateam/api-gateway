package file

import (
	"context"
	"fmt"
	"sync"

	"github.com/meateam/api-gateway/internal/util"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	upb "github.com/meateam/upload-service/proto"
	pool "github.com/processout/grpc-go-pool"
	"github.com/sirupsen/logrus"
)

// DeleteFile deletes fileID from file service and upload service, returns a slice of IDs of the files
// that were deleted if there were any files that are descendants of fileID and any error if occurred.
func DeleteFile(ctx context.Context,
	logger *logrus.Logger,
	fileConnPool *pool.Pool,
	uploadConnPool *pool.Pool,
	fileID string) ([]string, error) {
	// IMPORTANT TODO: need to check permissions per file that descends from fileID.
	deleteFileRequest := &fpb.DeleteFileRequest{
		Id: fileID,
	}

	fileClient, fileClientConn, err := util.GetFileClient(ctx, fileConnPool)
	if err != nil {
		return nil, err
	}
	defer fileClientConn.Close()

	uploadClient, uploadClientConn, err := util.GetUploadClient(ctx, uploadConnPool)
	if err != nil {
		return nil, err
	}
	defer uploadClientConn.Close()

	deleteFileResponse, err := fileClient.DeleteFile(ctx, deleteFileRequest)
	if err != nil {
		fileClientConn.Unhealthy()

		return nil, err
	}

	bucketKeysMap := make(map[string][]string)
	ids := make([]string, len(deleteFileResponse.GetFiles()))
	for i, file := range deleteFileResponse.GetFiles() {
		bucketKeysMap[file.GetBucket()] = append(bucketKeysMap[file.GetBucket()], file.GetKey())
		ids[i] = file.GetId()
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
			if err != nil {
				uploadClientConn.Unhealthy()

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
