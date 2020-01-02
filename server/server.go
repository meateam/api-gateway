package server

import (
	"net/http"

	ilogger "github.com/meateam/elasticsearch-logger"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	envPrefix                   = "GW"
	configPort                  = "port"
	configUploadService         = "upload_service"
	configDownloadService       = "download_service"
	configFileService           = "file_service"
	configUserService           = "user_service"
	configPermissionService     = "permission_service"
	configSearchService         = "search_service"
	configGotenbergService      = "gotenberg_service"
	configSecret                = "secret"
	configAuthURL               = "auth_url"
	configExternalApmURL        = "external_apm_url"
	configAllowOrigins          = "allow_origins"
	configSupportLink           = "support_link"
	configDownloadChromeURL     = "chrome_download_url"
	configElasticsearchURL      = "elasticsearch_url"
	configElasticsearchUser     = "elasticsearch_user"
	configElasticsearchPassword = "elasticsearch_password"
	configElasticsearchIndex    = "elasticsearch_index"
	configTLSSkipVerify         = "tls_skip_verify"
	configElasticsearchSniff    = "elasticsearch_sniff"
	configHealthCheckInterval   = "health_check_interval"
	configHealthCheckRPCTimeout = "health_check_rpc_timeout"
)

var (
	logger = ilogger.NewLogger()
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetDefault(configDownloadService, "download-service:8080")
	viper.SetDefault(configFileService, "file-service:8080")
	viper.SetDefault(configUserService, "user-service:8080")
	viper.SetDefault(configPermissionService, "permission-service:8080")
	viper.SetDefault(configSearchService, "search-service:8080")
	viper.SetDefault(configGotenbergService, "gotenberg-service:8080")
	viper.SetDefault(configSecret, "pandora@drive")
	viper.SetDefault(configAuthURL, "http://localhost/auth/login")
	viper.SetDefault(configExternalApmURL, "http://localhost:8200")
	viper.SetDefault(configAllowOrigins, "http://localhost*")
	viper.SetDefault(configSupportLink, "https://open.rocket.chat")
	viper.SetDefault(configElasticsearchURL, "http://localhost:9200")
	viper.SetDefault(configElasticsearchUser, "")
	viper.SetDefault(configElasticsearchPassword, "")
	viper.SetDefault(configElasticsearchIndex, "metrics")
	viper.SetDefault(configTLSSkipVerify, true)
	viper.SetDefault(configElasticsearchSniff, false)
	viper.SetDefault(configHealthCheckInterval, 5)
	viper.SetDefault(configHealthCheckRPCTimeout, 5)
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

// Server is a structure that holds the http server of the api-gateway.
type Server struct {
	server *http.Server
	conns  []*grpc.ClientConn
}

// NewServer creates a Server of the api-gateway.
func NewServer() *Server {
	router, conns := NewRouter(logger)

	s := &http.Server{
		Addr:           ":" + viper.GetString(configPort),
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
	}

	return &Server{server: s, conns: conns}
}

// Listen listens on configPort. Listen returns when listener is closed.
// Listener will be closed when this method returns, if listener is closed with non-nil
// error then it will be logged as fatal.
func (s *Server) Listen() {
	defer func() {
		for _, v := range s.conns {
			v.Close()
		}
	}()

	logger.Infof("server listening on port: %s", viper.GetString(configPort))
	if err := s.server.ListenAndServe(); err != nil {
		logger.Fatalf("%v", err)
	}
}
