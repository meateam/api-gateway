package main

import (
	"github.com/gin-gonic/gin"
)

type routerSetup interface {
	setup(r *gin.Engine)
}

func setupRouter() *gin.Engine {
	// Disable Console Color
	gin.DisableConsoleColor()
	r := gin.Default()
	u := uploadRouter{}

	u.setup(r)

	return r
}
