package server

import (
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/delegation"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/permission"
	"github.com/meateam/api-gateway/permit"
	"github.com/meateam/api-gateway/quota"
	"github.com/meateam/api-gateway/search"
	"github.com/meateam/api-gateway/server/auth"
	"github.com/meateam/api-gateway/upload"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/gotenberg-go-client/v6"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.elastic.co/apm/module/apmgin"
	"go.elastic.co/apm/module/apmgrpc"
	"go.elastic.co/apm/module/apmhttp"
	"google.golang.org/grpc"
)

const (
	healthcheckRoute  = "/api/healthcheck"
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
		gin.LoggerWithWriter(gin.DefaultWriter, healthcheckRoute),
		gin.Recovery(),
		apmgin.Middleware(r),
		cors.New(corsRouterConfig()),
		// Elasticsearch logger middleware.
		loggermiddleware.SetLogger(
			&loggermiddleware.Config{
				Logger:             logger,
				SkipPath:           []string{healthcheckRoute},
				SkipBodyPathRegexp: regexp.MustCompile(uploadRouteRegexp),
			},
		),
	)

	apiRoutesGroup := r.Group("/api")

	// Frontend configuration route.
	apiRoutesGroup.GET("/config", func(c *gin.Context) {
		c.JSON(
			http.StatusOK,
			gin.H{
				"chromeDownloadLink":   viper.GetString(configDownloadChromeURL),
				"apmServerUrl":         viper.GetString(configExternalApmURL),
				"environment":          os.Getenv("ELASTIC_APM_ENVIRONMENT"),
				"authUrl":              viper.GetString(configAuthURL),
				"docsUrl":              viper.GetString(configDocsURL),
				"supportLink":          viper.GetString(configSupportLink),
				"dropboxSupportLink":  viper.GetString(configDropboxSupportLink),
				"approvalServiceUrl":   viper.GetString(configApprovalServiceURL),
				"externalShareName":    viper.GetString(configExternalShareName),
				"myExternalSharesName": viper.GetString(configMyExternalSharesName),
				"vipServiceUrl":        viper.GetString(configVipService),
				"enableExternalShare":  viper.GetString(configEnableExternalShare),
				"whiteListText":  viper.GetString(configWhiteListText),
			},
		)
	})

	// Initiate services gRPC connections.
	delegateConn, err := initServiceConn(viper.GetString(configDelegationService))
	if err != nil {
		logger.Fatalf("couldn't setup delegation service connection: %v", err)
	}

	fileConn, err := initServiceConn(viper.GetString(configFileService))
	if err != nil {
		logger.Fatalf("couldn't setup file service connection: %v", err)
	}

	userConn, err := initServiceConn(viper.GetString(configUserService))
	if err != nil {
		logger.Fatalf("couldn't setup user service connection: %v", err)
	}

	uploadConn, err := initServiceConn(viper.GetString(configUploadService))
	if err != nil {
		logger.Fatalf("couldn't setup upload service connection: %v", err)
	}

	downloadConn, err := initServiceConn(viper.GetString(configDownloadService))
	if err != nil {
		logger.Fatalf("couldn't setup download service connection: %v", err)
	}

	permissionConn, err := initServiceConn(viper.GetString(configPermissionService))
	if err != nil {
		logger.Fatalf("couldn't setup permission service connection: %v", err)
	}

	permitConn, err := initServiceConn(viper.GetString(configPermitService))
	if err != nil {
		logger.Fatalf("couldn't setup permit service connection: %v", err)
	}

	searchConn, err := initServiceConn(viper.GetString(configSearchService))
	if err != nil {
		logger.Fatalf("couldn't setup search service connection: %v", err)
	}

	spikeConn, err := initServiceConn(viper.GetString(configSpikeService))
	if err != nil {
		logger.Fatalf("couldn't setup spike service connection: %v", err)
	}

	gotenbergClient := &gotenberg.Client{Hostname: viper.GetString(configGotenbergService)}

	// initiate middlewares
	om := oauth.NewOAuthMiddleware(spikeConn, delegateConn, logger)
	conns := []*grpc.ClientConn{
		fileConn,
		uploadConn,
		downloadConn,
		permissionConn,
		userConn,
		searchConn,
		spikeConn,
		permitConn,
		delegateConn,
	}

	health := NewHealthChecker()
	healthInterval := viper.GetInt(configHealthCheckInterval)
	healthRPCTimeout := viper.GetInt(configHealthCheckRPCTimeout)

	go health.Check(healthInterval, healthRPCTimeout, logger, gotenbergClient, conns...)

	// Health Check route.
	apiRoutesGroup.GET("/healthcheck", health.healthCheck)

	// Initiate routers.
	dr := delegation.NewRouter(delegateConn, logger)
	fr := file.NewRouter(fileConn, downloadConn, uploadConn, permissionConn, permitConn,
		searchConn, userConn, delegateConn, gotenbergClient, om, logger)
	ur := upload.NewRouter(uploadConn, fileConn, permissionConn, searchConn, om, logger)
	usr := user.NewRouter(userConn, logger)
	ar := auth.NewRouter(logger)
	qr := quota.NewRouter(fileConn, logger)
	pr := permission.NewRouter(permissionConn, fileConn, userConn, om, logger)
	ptr := permit.NewRouter(permitConn, permissionConn, fileConn, delegateConn, om, logger)
	sr := search.NewRouter(searchConn, fileConn, permissionConn, logger)

	middlewares := make([]gin.HandlerFunc, 0, 2)

	secrets := auth.Secrets{
		Drive: viper.GetString(configSecret),
		Docs:  viper.GetString(configDocsSecret),
	}

	authRequiredMiddleware := ar.Middleware(secrets, viper.GetString(configAuthURL))
	middlewares = append(middlewares, authRequiredMiddleware)

	if metricsLogger := NewMetricsLogger(); metricsLogger != nil {
		middlewares = append(middlewares, metricsLogger)
	}
	// Authentication middleware on routes group.
	authRequiredRoutesGroup := apiRoutesGroup.Group("/", middlewares...)

	// Initiate client connection to delegation service.
	dr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to file service.
	fr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to user service.
	usr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to quota service.
	qr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to upload service.
	ur.Setup(authRequiredRoutesGroup)

	// Initiate client connection to permission service.
	pr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to permit service.
	ptr.Setup(authRequiredRoutesGroup)

	// Initiate client connection to search service.
	sr.Setup(authRequiredRoutesGroup)

	// Create a slice to manage connections and return it.
	return r, conns
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
