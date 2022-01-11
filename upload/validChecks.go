package upload

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/user"
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


// getQueryFromContextWhitAbort extracts the query from the context
func (r *Router) getQueryFromContext(c *gin.Context, query string) (string, bool) {
	queryRes, exists := c.GetQuery(query)
	if !exists {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s is required", query))
		return "", false
	}
	return queryRes, true
}
