package upload

import (
	"fmt"
	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"
	"net/http"
)

// isUserFromContext check if can extracts the user's details from c.
func (r *Router) getUserFromContext(c *gin.Context) *user.User {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("error extracting user from request")),
		)
		return nil
	}
	return reqUser
}

// isUploadPermitted checks if userID has permission to upload a file to fileID,
// requires ppb.Role_WRITE permission.
func (r *Router) isUploadPermittedForUser(c *gin.Context, userID string, fileID string) bool {
	userFilePermission, _, err := file.CheckUserFilePermission(
		c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		userID,
		fileID,
		UploadRole)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return false
	}
	if userFilePermission == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return false
	}
	return true
}

// isQueryInContext check if has query in context
func (r *Router) getQueryFromContext(c *gin.Context, query string) (string, bool) {
	queryRes, exists := c.GetQuery(query)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", query))
		return "", false
	}
	return queryRes, true
}

// abortWithError is abort with error
func (r *Router) abortWithError(c *gin.Context, err error) {
	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
}
