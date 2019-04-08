package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type uploadRouter struct {
	uploadServiceURL string
}

func (ur uploadRouter) setup(r *gin.Engine) {
	r.GET("/upload", ur.upload)
}

func (ur uploadRouter) upload(c *gin.Context) {
	c.String(http.StatusOK, "not implemented")
}
