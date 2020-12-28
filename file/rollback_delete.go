package file

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
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
	// TODO : do i need to check for user id?

	var wg sync.WaitGroup
	defer wg.Wait()
	for _, permission := range permissions {
		wg.Add(1)
		go func(permission *ppb.GetFilePermissionsResponse_UserRole) {

			// TODO : check what is override in create permissions
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
