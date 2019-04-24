package main

import (
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func setupRouter() (r *gin.Engine, close func()) {
	const numOfRPCConns = 3

	// Disable Console Color
	gin.DisableConsoleColor()
	r = gin.Default()

	// Default cors handeling.
	corsConfig := cors.DefaultConfig()
	corsConfig.AddExposeHeaders("x-uploadid")
	corsConfig.AddAllowHeaders("cache-control", "x-requested-with", "content-disposition", "content-range")
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	// Auth middleware
	r.Use(authRequired)

	// Initiate file router.
	fr := &fileRouter{
		fileServiceURL: viper.GetString(configfileService),
	}

	// Initiate upload router.
	ur := &uploadRouter{
		uploadServiceURL: viper.GetString(configUploadService),
	}

	// Initiate download router.
	dr := &downloadRouter{
		downloadServiceURL: viper.GetString(configDownloadService),
	}

	// Creating a slice to manage connections
	conns := make([]*grpc.ClientConn, 0, numOfRPCConns)

	// Initiate client connection to file service.
	// Appends The connection to the connections slice.
	fconn, err := fr.setup(r)
	if err != nil {
		log.Fatalf("couldn't setup upload router: %v", err)
	}
	conns = append(conns, fconn)

	// Initiate client connection to upload service.
	// Appends The connection to the connections slice.
	uconn, err := ur.setup(r, fconn)
	if err != nil {
		log.Fatalf("couldn't setup upload router: %v", err)
	}
	conns = append(conns, uconn)

	// Initiate client connection to download service.
	// Appends The connection to the connections slice.
	dconn, err := dr.setup(r, fconn)
	if err != nil {
		log.Fatalf("couldn't setup download router: %v", err)
	}
	conns = append(conns, dconn)

	// Defines a function that is closing all connections in order to defer it outside.
	close = func() {
		for _, v := range conns {
			v.Close()
		}
	}
	return
}

func authRequired(c *gin.Context) {
	c.Set("User", user{id: "testuser"})
}
