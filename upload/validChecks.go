package upload

import (
	"fmt"
	"net/http"
	"google.golang.org/grpc/status"
	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
	ppb "github.com/meateam/permission-service/proto"
)

// getUserFromContext extracts the user from the context.
func (r *Router) getUserFromContext(c *gin.Context) *user.User {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("error extracting user from request")))
		return nil
	}
	return reqUser
}

// HandleUserFilePermission gets a gin context, the requested file id, and the role the user needs.
// Returns true if the user was shared to the file.
// Returns false and aborts with status if the user isn't permitted to operate on it,
// Returns false if any error occurred and logs the error.
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
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return false
	}

	// If no permission is returned it means there is no permission to do the action
	if userFilePermission == "" {
		loggermiddleware.LogError(r.logger, c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("You do not have permission to do this operation")))
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

