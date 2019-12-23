package oauth

import (
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
)

const (

	// ScopesKey  is the context key used to get the service scopes in the context
	ScopesKey = "Scopes"

	// DelegatorKey is the context key used to get and set the delegator's data in the context.
	DelegatorKey = "Delegator"

	// OutAdminScope is the scope name required for uploading,
	// downloading, and sharing files for an out-source user
	OutAdminScope = "read"

	// updatePermitStatusScope is the scope name required for updating a permit's scope
	updatePermitStatusScope = "status"
)

// CheckScope check the scopes in context. If scopes are nil, the client is the drive client
// which is fine. Else, the required scope should be included in the scopes array.
// If the required scope exists, and a delegator exists too, the function will set the context
// user to be the delegator.
func CheckScope(c *gin.Context, requiredScope string) bool {
	contextScopes := c.Value(ScopesKey)
	if contextScopes == nil {
		return true
	}
	switch scopes := contextScopes.(type) {
	case []string:
		for _, scope := range scopes {
			if scope == requiredScope {
				break
			}
			return false
		}
	default:
		return false
	}
	DelegatorToUser(c)
	return true
}

// DelegatorToUser copies the delegator user to the user in context.
// if the delegator does not exist it doest nothing.
func DelegatorToUser(c *gin.Context) {
	contextDelegator := c.Value(DelegatorKey)
	if contextDelegator == nil {
		return
	}
	switch delegator := contextDelegator.(type) {
	case user.User:
		c.Set(user.ContextUserKey, delegator)
	default:
		return
	}
}
