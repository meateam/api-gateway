package upload

import (
	ppb "github.com/meateam/permission-service/proto"
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	"google.golang.org/grpc/status"
)

// getUserFromContext extracts the user from the context.
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

// isUploadPermittedForUser checks if userID has permission to upload a file to folder,
// requires ppb.Role_WRITE permission.
func (r *Router) isUploadPermittedForUser(c *gin.Context, userID string, parentID string) bool {
	userFilePermission, _, err := file.CheckUserFilePermission(
		c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		userID,
		parentID,
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

	userFilePermission, foundPermission, err := file.CheckUserFilePermission(c.Request.Context(),
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

// getQueryFromContext extracts the query from the context
func (r *Router) getQueryFromContextWhitAbort(c *gin.Context, query string) (string, bool) {
	queryRes, exists := c.GetQuery(query)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", query))
		return "", false
	}
	return queryRes, true
}

// getQueryFromContext extracts the query from the context
func (r *Router) getQueryFromContext(c *gin.Context, query string) (string, bool) {
	return c.GetQuery(query)
}

// abortWithError is abort with error
func (r *Router) abortWithError(c *gin.Context, err error) {
	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
}
