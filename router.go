package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/spf13/viper"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgin"
	"go.elastic.co/apm/module/apmgrpc"
	"go.elastic.co/apm/module/apmhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func setupRouter() (r *gin.Engine, close func()) {
	// Disable Console Color
	gin.DisableConsoleColor()
	r = gin.Default()
	r.Use(apmgin.Middleware(r))

	// Default cors handeling.
	corsConfig := cors.DefaultConfig()
	corsConfig.AddExposeHeaders("x-uploadid")
	corsConfig.AddAllowHeaders(
		"cache-control",
		"x-requested-with",
		"content-disposition",
		"content-range",
		apmhttp.TraceparentHeader,
	)
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	r.Use(
		loggermiddleware.SetLogger(
			&loggermiddleware.Config{
				Logger:   logger,
				SkipPath: []string{"/healthcheck"},
			},
		),
		gin.Recovery(),
	)

	// Authentication middleware
	r.Use(authRequired)

	// Health Check route
	r.GET("/healthcheck", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Initiate file router.
	fileConn, err := initServiceConn(viper.GetString(configfileService))
	if err != nil {
		logger.Fatalf("couldn't setup file service connection: %v", err)
	}

	uploadConn, err := initServiceConn(viper.GetString(configUploadService))
	if err != nil {
		logger.Fatalf("couldn't setup upload service connection: %v", err)
	}

	downloadConn, err := initServiceConn(viper.GetString(configDownloadService))
	if err != nil {
		logger.Fatalf("couldn't setup download service connection: %v", err)
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
	conn, err := grpc.Dial(url,
		grpc.WithUnaryInterceptor(apmgrpc.NewUnaryClientInterceptor()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)),
		grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func authRequired(c *gin.Context) {
	c.Set("User", user{id: "testuser"})
}

// startSpan starts an "external.grpc" span under the transaction in ctx,
// returns the created span and the context with the traceparent header matadata.
func startSpan(ctx context.Context, name string) (*apm.Span, context.Context) {
	span, ctx := apm.StartSpan(ctx, name, "external.grpc")
	if span.Dropped() {
		return span, ctx
	}
	traceparentValue := apmhttp.FormatTraceparentHeader(span.TraceContext())
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.Pairs(strings.ToLower(apmhttp.TraceparentHeader), traceparentValue)
	} else {
		md = md.Copy()
		md.Set(strings.ToLower(apmhttp.TraceparentHeader), traceparentValue)
	}
	return span, metadata.NewOutgoingContext(ctx, md)
}
