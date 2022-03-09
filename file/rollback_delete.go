package file

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
)

// AddPermissionsOnError add the deleted permissions when the file is failed to delete
func AddPermissionsOnError(c *gin.Context,
	err error,
	fileID string,
	permissions []*ppb.GetFilePermissionsResponse_UserRole,
	permissionClient ppb.PermissionClient,
	logger *logrus.Logger) {

	var wg sync.WaitGroup
	defer wg.Wait()
	for _, permission := range permissions {
		wg.Add(1)
		go func(permission *ppb.GetFilePermissionsResponse_UserRole) {
			permissionRequest := &ppb.CreatePermissionRequest{
				FileID:  fileID,
				UserID:  permission.GetUserID(),
				Role:    permission.GetRole(),
				Creator: permission.GetCreator(),
			}

			_, err := permissionClient.CreatePermission(c, permissionRequest)
			if err != nil {
				httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
				loggermiddleware.LogError(logger, c.AbortWithError(httpStatusCode,
					fmt.Errorf("failed rollback and recreate permissions for file: %s", fileID)))

				return
			}

			wg.Done()
		}(permission)
	}

	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	loggermiddleware.LogError(logger, c.AbortWithError(httpStatusCode, err))
}

func DeletePermissionOnError(c *gin.Context,
	fileID string,
	userID string,
	permissionClient ppb.PermissionClient,
	logger *logrus.Logger) {
	
	permissionRequest := &ppb.DeletePermissionRequest{
		FileID: fileID,
		UserID: userID,
	}

	_, err := permissionClient.DeletePermission(c, permissionRequest)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(logger, c.AbortWithError(httpStatusCode,
			fmt.Errorf("failed rollback and delete permissions for file: %s", fileID)))

		return
	}
}


func DeleteFileOnError(c *gin.Context,
	fileID string,
	userID string,
	fileClient fpb.FileServiceClient,
	permissionClient ppb.PermissionClient,
	logger *logrus.Logger) {
		deletedFile, err := fileClient.DeleteFileByID(c, &fpb.DeleteFileByIDRequest{Id: fileID})
		if err != nil || deletedFile == nil {
			httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))		
			loggermiddleware.LogError(logger, c.AbortWithError(httpStatusCode, fmt.Errorf("failed deleting file: %v", err)))
		}
	}
