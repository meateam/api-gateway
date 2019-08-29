package server

import (
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/quota"
	"github.com/meateam/api-gateway/server/auth"
	"github.com/meateam/api-gateway/upload"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.elastic.co/apm/module/apmgin"
	"go.elastic.co/apm/module/apmgrpc"
	"go.elastic.co/apm/module/apmhttp"
	"google.golang.org/grpc"
)

const (
	healtcheckRouter  = "/api/healtcheck"
	uploadRouteRegexp = "/api/upload.+"
)

// NewRouter creates new gin.Engine for the api-gateway server and sets it up.
func NewRouter(logger *logrus.Logger) (*gin.Engine, []*grpc.ClientConn) {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	gin.DisableConsoleColor()
	r := gin.New()

	// Setup logging, metrics, cors middlewares.
	r.Use(
		// Ignore logging healthcheck routes.
		gin.LoggerWithWriter(gin.DefaultWriter, healtcheckRouter),
		gin.Recovery(),
		apmgin.Middleware(r),
		cors.New(corsRouterConfig()),
		// Elasticsearch logger middleware.
		loggermiddleware.SetLogger(
			&loggermiddleware.Config{
				Logger:             logger,
				SkipPath:           []string{healtcheckRouter},
				SkipBodyPathRegexp: regexp.MustCompile(uploadRouteRegexp),
			},
		),
	)

	apiRoutesGroup := r.Group("/api")

	// Health Check route.
	apiRoutesGroup.GET("/healthcheck", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Frontend configuration route.
	apiRoutesGroup.GET("/config", func(c *gin.Context) {
		c.JSON(
			http.StatusOK,
			gin.H{
				"chromeDownloadLink": viper.GetString(configDownloadChromeURL),
				"apmServerUrl":       viper.GetString(configExternalApmURL),
				"environment":        os.Getenv("ELASTIC_APM_ENVIRONMENT"),
				"authUrl":            viper.GetString(configAuthURL),
				"supportLink":        viper.GetString(configSupportLink),
			},
		)
	})

	// Initiate services gRPC connections.
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

	// Initiate routers.
	fr := file.NewRouter(fileConn, downloadConn, uploadConn, logger)
	ur := upload.NewRouter(uploadConn, fileConn, logger)
	ar := auth.NewRouter(logger)
	qr := quota.NewRouter(fileConn, logger)

	// Authentication middleware on routes group.
	authRequiredMiddleware := ar.Middleware(viper.GetString(configSecret), viper.GetString(configAuthURL))
	authRequiredRoutesGroup := apiRoutesGroup.Group("/", authRequiredMiddleware)

	// Initiate client connection to file service.
	fr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to quota service.
	qr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to upload service.
	ur.Setup(authRequiredRoutesGroup)

	// Create a slice to manage connections and return it.
	return r, []*grpc.ClientConn{fileConn, uploadConn, downloadConn}
}

// corsRouterConfig configures cors policy for cors.New gin middleware.
func corsRouterConfig() cors.Config {
	corsConfig := cors.DefaultConfig()
	corsConfig.AddExposeHeaders("x-uploadid")
	corsConfig.AllowAllOrigins = false
	corsConfig.AllowWildcard = true
	corsConfig.AllowOrigins = strings.Split(viper.GetString(configAllowOrigins), ",")
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders(
		"x-content-length",
		"authorization",
		"cache-control",
		"x-requested-with",
		"content-disposition",
		"content-range",
		apmhttp.TraceparentHeader,
	)

	return corsConfig
}

// initServiceConn creates a gRPC connection to url, returns the created connection
// and nil err on success. Returns non-nil error if any error occurred while
// creating the connection.
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
