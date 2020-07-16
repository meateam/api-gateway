package upload

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	ppb "github.com/meateam/permission-service/proto"
	"google.golang.org/grpc/status"
)

// getUserFromContext extracts the user from the context.
func (r *Router) getUserFromContext(c *gin.Context) *user.User {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		r.LogE(c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("error extracting user from request")))
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
		UploadRole,
	)

	if err != nil {
		r.abortWithHttpStatusByError(c, err)
		return false
	}

	// If no permission is returned it means there is no permission to do the action
	if userFilePermission == "" {
		r.LogE(c.AbortWithError(http.StatusForbidden, fmt.Errorf("Upload blocked, no permission")))
		return false
	}
	return true
}

// HandleUserFilePermission gets a gin context, the requested file id, and the role the user needs.
// Returns the user file permission-string and the permission object if the user was shared to the file.
// Returns an empty string and nil and aborts with status if the user isn't permitted to operate on it,
// Returns an empty string and nil if any error occurred and logs the error.
func (r *Router) HandleUserFilePermission(c *gin.Context, fileID string, role ppb.Role) bool {
	reqUser := r.getUserFromContext(c)
	if reqUser == nil {
		return false
	}

	userFilePermission, _, err := file.CheckUserFilePermission(
		c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		fileID,
		role,
	)

	if err != nil {
		r.abortWithHttpStatusByError(c, err)
		return false
	}

	// If no permission is returned it means there is no permission to do the action
	if userFilePermission == "" {
		r.LogE(c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("You do not have permission to do this operation")))
	}

	return true
}

// getQueryFromContextWhitAbort extracts the query from the context
func (r *Router) getQueryFromContext(c *gin.Context, query string) (string, bool) {
	queryRes, exists := c.GetQuery(query)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", query))
		return "", false
	}
	return queryRes, true
}

// abortWithError is abort with error
func (r *Router) abortWithHttpStatusByError(c *gin.Context, err error) {
	httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
	r.LogE(c.AbortWithError(httpStatusCode, err))
}

// LogE log to the console error
func (r *Router) LogE(err error) {
	loggermiddleware.LogError(r.logger, err)
}
