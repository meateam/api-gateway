package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-openapi/runtime/middleware"
	"github.com/meateam/api-gateway/dropbox"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/permission"
	"github.com/meateam/api-gateway/quota"
	"github.com/meateam/api-gateway/search"
	"github.com/meateam/api-gateway/server/auth"
	"github.com/meateam/api-gateway/upload"
	"github.com/meateam/api-gateway/user"
	"github.com/meateam/gotenberg-go-client/v6"
	grpcPool "github.com/meateam/grpc-go-conn-pool/grpc"
	grpcPoolOptions "github.com/meateam/grpc-go-conn-pool/grpc/options"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
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

// ExternalNetworkDest configuration of external network
type ExternalNetworkDest struct {
	Label          string `json:"label"`
	Value          string `json:"value"`
	AppID          string `json:"appID"`
	ApprovalURL    string `json:"approvalUrl"`
	ApprovalUIURL  string `json:"approvalUIUrl"`
	IsDefault      bool   `json:"isDefault"`
	IsEnabled      bool   `json:"isEnabled"`
	IsOnlyApprover bool   `json:"isOnlyApprover"`
}

// NewRouter creates new gin.Engine for the api-gateway server and sets it up.
func NewRouter(logger *logrus.Logger) (*gin.Engine, []*grpcPoolTypes.ConnPool) {
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
				"authUrl":              viper.GetString(configAuthURL),
				"docsUrl":              viper.GetString(configDocsURL),
				"supportLink":          viper.GetString(configSupportLink),
				"dropboxSupportLink":   viper.GetString(configDropboxSupportLink),
				"externalShareName":    viper.GetString(configExternalShareName),
				"myExternalSharesName": viper.GetString(configMyExternalSharesName),
				"vipServiceUrl":        viper.GetString(configVipService),
				"enableExternalShare":  viper.GetString(configEnableExternalShare),
				"whiteListText":        viper.GetString(configWhiteListText),
				"bereshitSupportLink":  viper.GetString(configBereshitSupportLink),
				"bamSupportNumber":     viper.GetString(configBamSupportNumber),
				"statusSuccessType":    viper.GetString(configTransferStatusSuccess),
				"statusFailedType":     viper.GetString(configTransferStatusFailed),
				"statusInProgressType": viper.GetString(configTransferStatusInProgress),
				"environment":          os.Getenv("ELASTIC_APM_ENVIRONMENT"),
				"externalNetworkDests": GetExternalNetworksConfiguration(),
				"localOfficeUrl":       viper.GetString(configLocalOfficeURL),
				"CTSSuffix":            viper.GetString(configCTSSuffix),
			},
		)
	})

	// Initiate services gRPC connections.
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

	dropboxConn, err := initServiceConn(viper.GetString(configDropboxService))
	if err != nil {
		logger.Fatalf("couldn't setup dropbox service connection: %v", err)
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
	om := oauth.NewOAuthMiddleware(spikeConn, userConn, logger)

	nonFatalConns := []*grpcPoolTypes.ConnPool{
		dropboxConn,
		userConn,
		spikeConn,
	}

	fatalConns := []*grpcPoolTypes.ConnPool{
		fileConn,
		downloadConn,
		permissionConn,
		uploadConn,
		searchConn,
	}

	conns := append(fatalConns, nonFatalConns...)

	health := NewHealthChecker()
	healthInterval := viper.GetInt(configHealthCheckInterval)
	healthRPCTimeout := viper.GetInt(configHealthCheckRPCTimeout)

	badConns := make(chan *grpcPoolTypes.ConnPool, len(conns))

	go health.Check(healthInterval, healthRPCTimeout, logger, gotenbergClient, badConns, nonFatalConns, fatalConns...)
	go reviveConns(badConns)

	// Health Check route.
	apiRoutesGroup.GET("/healthcheck", health.healthCheck)

	// handler for swagger documentation
	apiRoutesGroup.StaticFile("/swagger", viper.GetString(configSwaggerPathFile)+"/swagger.json")

	if viper.GetBool(configShowSwaggerUI) {
		opts := middleware.SwaggerUIOpts{
			SpecURL:  "/api/swagger",
			BasePath: "/api",
		}

		sh := middleware.SwaggerUI(opts, nil)
		apiRoutesGroup.GET("/docs", gin.WrapH(sh))
	} else {
		opts := middleware.RedocOpts{
			SpecURL:  "/api/swagger",
			BasePath: "/api",
			RedocURL: "/api/redoc",
		}

		// redoc UI
		apiRoutesGroup.StaticFile("/redoc", viper.GetString(configSwaggerPathFile)+"/redoc.standalone.js")

		sh := middleware.Redoc(opts, nil)
		apiRoutesGroup.GET("/docs", gin.WrapH(sh))
	}

	// Initiate routers.
	fr := file.NewRouter(fileConn, downloadConn, uploadConn, permissionConn, dropboxConn,
		searchConn, gotenbergClient, om, logger)
	ur := upload.NewRouter(uploadConn, fileConn, permissionConn, searchConn, om, logger)
	usr := user.NewRouter(userConn, logger)
	ar := auth.NewRouter(logger)
	qr := quota.NewRouter(fileConn, logger)
	pr := permission.NewRouter(permissionConn, fileConn, userConn, om, logger)
	drp := dropbox.NewRouter(dropboxConn, permissionConn, om, logger)
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

	// Initiate client connection to dropbox service.
	drp.Setup(authRequiredRoutesGroup)

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
		"destination",
		"fileID",
		apmhttp.TraceparentHeader,
	)

	return corsConfig
}

// initServiceConn creates a gRPC connection pool to url, returns the created connection pool
// and nil err on success. Returns non-nil error if any error occurred while
// creating the connection pool.
func initServiceConn(url string) (*grpcPoolTypes.ConnPool, error) {
	ctx := context.Background()
	connPool, err := grpcPool.DialPool(ctx,
		grpcPoolOptions.WithGRPCDialOption(grpc.WithUnaryInterceptor(apmgrpc.NewUnaryClientInterceptor())),
		grpcPoolOptions.WithGRPCDialOption(grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20))),
		grpcPoolOptions.WithGRPCDialOption(grpc.WithInsecure()),
		grpcPoolOptions.WithEndpoint(url),
		grpcPoolOptions.WithGRPCConnectionPool(viper.GetInt(configPoolSize)),
	)
	if err != nil {
		return nil, err
	}
	return &connPool, nil
}

// reviveConns is the function for reviving the connections in the badConns connection pool
func reviveConns(badConns <-chan *grpcPoolTypes.ConnPool) {
	for {
		// Pull the pointer to the pool from the channel.
		// Will run when the channel isn't empty
		pool := <-badConns
		go func(pool *grpcPoolTypes.ConnPool) {
			// Get the target url
			target := (*pool).Conn().Target()
			var newPool *grpcPoolTypes.ConnPool
			err := fmt.Errorf("temp")
			for err != nil {
				time.Sleep(time.Second * time.Duration(2))
				err = nil
				newPool, err = initServiceConn(target)
			}
			(*pool).Close()
			// Replace the pointer to the new pool
			*pool = *newPool
		}(pool)
	}
}

// GetExternalNetworksConfiguration get network object configuration
func GetExternalNetworksConfiguration() []ExternalNetworkDest {
	var externalNetworkDests = []ExternalNetworkDest{
		{
			Value:          viper.GetString(configTomcalDestValue),
			Label:          viper.GetString(configTomcalDestName),
			AppID:          viper.GetString(configTomcalDestAppID),
			ApprovalURL:    viper.GetString(configApprovalServiceURL),
			ApprovalUIURL:  viper.GetString(configApprovalServiceUIURL),
			IsDefault:      true,
			IsEnabled:      viper.GetBool(configTomcalDestEnabled),
			IsOnlyApprover: viper.GetBool(configTomcalDestOnlyApprover),
		},
		{
			Value:          viper.GetString(configCtsDestValue),
			Label:          viper.GetString(configCtsDestName),
			AppID:          viper.GetString(configCtsDestAppID),
			ApprovalURL:    viper.GetString(configApprovalCtsServiceURL),
			ApprovalUIURL:  viper.GetString(configApprovalCtsServiceUIURL),
			IsDefault:      false,
			IsEnabled:      viper.GetBool(configCtsDestEnabled),
			IsOnlyApprover: viper.GetBool(configCtsDestOnlyApprover),
		},
	}

	return externalNetworkDests
}
