package user

import "github.com/gin-gonic/gin"

const (
	// ContextUserKey is the context key used to get and set the user's data in the context.
	ContextUserKey = "User"
)

// User is a structure of an authenticated user.
type User struct {
	ID        string
	FirstName string
	LastName  string
}

// ExtractRequestUser gets a gin.Context and extracts the
func ExtractRequestUser(c *gin.Context) *User {
	contextUser, exists := c.Get(ContextUserKey)
	if !exists {
		return nil
	}

	var reqUser User
	switch v := contextUser.(type) {
	case User:
		reqUser = v
	default:
		return nil
	}

	return &reqUser
}
