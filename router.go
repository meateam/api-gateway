package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

type routerSetup interface {
	setup(r *gin.Engine) (*grpc.ClientConn, error)
}

func setupRouter() (*gin.Engine, func()) {
	const numOfRPCConns = 1
	// Disable Console Color
	gin.DisableConsoleColor()
	r := gin.Default()
	r.MaxMultipartMemory = 5 << 20
	r.Use(authRequired)
	u := &uploadRouter{
		uploadServiceURL: viper.GetString(configUploadService),
	}
	conns := make([]*grpc.ClientConn, 0, numOfRPCConns)
	uconn, err := u.setup(r)
	if err != nil {
		log.Fatalf("couldn't setup upload router: %v", err)
	}

	conns = append(conns, uconn)

	close := func() {
		for _, v := range conns {
			v.Close()
		}
	}

	return r, close
}

func authRequired(c *gin.Context) {
	c.Set("User", user{id: "testuser"})
}
