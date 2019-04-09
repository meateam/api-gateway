package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// routerSetup is an interface for setting up a *gin.Engine with routes, middlewares,
// groups, etc, and a connection to a RPC service that will be used under the router.
type routerSetup interface {
	// setup gets a *gin.Engine and sets up its routes, middlewares, groups, etc,
	// and returns a *grpc.ClientConn to a RPC service.
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
