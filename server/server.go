package server

import (
	"net/http"

	ilogger "github.com/meateam/elasticsearch-logger"
	pool "github.com/processout/grpc-go-pool"
	"github.com/spf13/viper"
)

const (
	envPrefix               = "GW"
	configPort              = "port"
	configUploadService     = "upload_service"
	configDownloadService   = "download_service"
	configFileService       = "file_service"
	configUserService       = "user_service"
	configPermissionService = "permission_service"
	configSecret            = "secret"
	configAuthURL           = "auth_url"
	configExternalApmURL    = "external_apm_url"
	configAllowOrigins      = "allow_origins"
	configSupportLink       = "support_link"
	configDownloadChromeURL = "chrome_download_url"
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
	viper.SetDefault(configSecret, "pandora@drive")
	viper.SetDefault(configAuthURL, "http://localhost/auth/login")
	viper.SetDefault(configExternalApmURL, "http://localhost:8200")
	viper.SetDefault(configAllowOrigins, "http://localhost*")
	viper.SetDefault(configSupportLink, "https://open.rocket.chat")
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

// Server is a structure that holds the http server of the api-gateway.
type Server struct {
	server    *http.Server
	connPools []*pool.Pool
}

// NewServer creates a Server of the api-gateway.
func NewServer() *Server {
	router, connPools := NewRouter(logger)

	s := &http.Server{
		Addr:           ":" + viper.GetString(configPort),
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
	}

	return &Server{server: s, connPools: connPools}
}

// Listen listens on configPort. Listen returns when listener is closed.
// Listener will be closed when this method returns, if listener is closed with non-nil
// error then it will be logged as fatal.
func (s *Server) Listen() {
	defer func() {
		for _, v := range s.connPools {
			v.Close()
		}
	}()

	logger.Infof("server listening on port: %s", viper.GetString(configPort))
	if err := s.server.ListenAndServe(); err != nil {
		logger.Fatalf("%v", err)
	}
}
