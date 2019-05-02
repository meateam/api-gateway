package main

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func setupRouter() (r *gin.Engine, close func()) {
	// Disable Console Color
	gin.DisableConsoleColor()
	r = gin.Default()

	// Default cors handeling.
	corsConfig := cors.DefaultConfig()
	corsConfig.AddExposeHeaders("x-uploadid")
	corsConfig.AddAllowHeaders("cache-control", "x-requested-with", "content-disposition", "content-range")
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	// Authentication middleware
	r.Use(authRequired)

	// Health Check route
	r.GET("/healthcheck", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Initiate file router.
	fileConn, err := initServiceConn(viper.GetString(configfileService))
	if err != nil {
		log.Fatalf("couldn't setup file service connection: %v", err)
	}

	uploadConn, err := initServiceConn(viper.GetString(configUploadService))
	if err != nil {
		log.Fatalf("couldn't setup upload service connection: %v", err)
	}

	downloadConn, err := initServiceConn(viper.GetString(configDownloadService))
	if err != nil {
		log.Fatalf("couldn't setup download service connection: %v", err)
	}

	// Initiate file router.
	fr := &fileRouter{}

	// Initiate upload router.
	ur := &uploadRouter{}

	// Initiate client connection to file service.
	fr.setup(r, fileConn, downloadConn)

	// Initiate client connection to upload service.
	ur.setup(r, uploadConn, fileConn)

	// Creating a slice to manage connections
	conns := []*grpc.ClientConn{fileConn, uploadConn, downloadConn}

	// Defines a function that is closing all connections in order to defer it outside.
	close = func() {
		for _, v := range conns {
			v.Close()
		}

		return
	}

	return
}

func initServiceConn(url string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(url, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func authRequired(c *gin.Context) {
	c.Set("User", user{id: "testuser"})
}
