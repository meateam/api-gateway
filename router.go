package main

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type routerSetup interface {
	setup(r *gin.Engine)
}

func setupRouter() *gin.Engine {
	// Disable Console Color
	gin.DisableConsoleColor()
	r := gin.Default()
	u := uploadRouter{
		uploadServiceURL: viper.GetString(configUploadService),
	}

	u.setup(r)

	return r
}
