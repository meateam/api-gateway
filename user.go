package main

import "github.com/gin-gonic/gin"

type user struct {
	id string
	firstName string
	lastName string
}

func extractRequestUser(c *gin.Context) *user {
	contextUser, exists := c.Get("User")
	if exists != true {
		return nil
	}

	var reqUser user
	switch v := contextUser.(type) {
	case user:
		reqUser = v
		break
	default:
		return nil
	}

	return &reqUser
}
