package server

import (
	"net/http"

	"github.com/meateam/api-gateway/server/auth"
	"github.com/meateam/api-gateway/user"
	ilogger "github.com/meateam/elasticsearch-logger"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	envPrefix                   = "GW"
	configPort                  = "port"
	configUploadService         = "upload_service"
	configDelegationService     = "delegation_service"
	configDocsSecret			= "docs_secret"
	configDownloadService       = "download_service"
	configFileService           = "file_service"
	configUserService           = "user_service"
	configPermissionService     = "permission_service"
	configPermitService         = "permit_service"
	configSearchService         = "search_service"
	configSpikeService          = "spike_service"
	configGotenbergService      = "gotenberg_service"
	configSecret                = "secret"
	configAuthURL               = "auth_url"
	configDocsURL               = "docs_url"
	configExternalApmURL        = "external_apm_url"
	configAllowOrigins          = "allow_origins"
	configSupportLink           = "support_link"
	configDropboxSupportLink    = "dropbox_support_link"
	configDownloadChromeURL     = "chrome_download_url"
	configElasticsearchURL      = "elasticsearch_url"
	configElasticsearchUser     = "elasticsearch_user"
	configElasticsearchPassword = "elasticsearch_password"
	configElasticsearchIndex    = "elasticsearch_index"
	configTLSSkipVerify         = "tls_skip_verify"
	configElasticsearchSniff    = "elasticsearch_sniff"
	configHealthCheckInterval   = "health_check_interval"
	configHealthCheckRPCTimeout = "health_check_rpc_timeout"
	configApprovalServiceURL    = "approval_url"
	configExternalShareName     = "external_share_name"
	configMyExternalSharesName  = "my_external_shares_name"
	configVipService            = "vip_service"
	configEnableExternalShare   = "enable_external_share"
	configWhiteListText = "white_list_text"
)

var (
	logger = ilogger.NewLogger()
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetDefault(configDelegationService, "delegation-service:8080")
	viper.SetDefault(configDocsSecret, "docs@drive")
	viper.SetDefault(configDownloadService, "download-service:8080")
	viper.SetDefault(configFileService, "file-service:8080")
	viper.SetDefault(configUserService, "user-service:8080")
	viper.SetDefault(configPermissionService, "permission-service:8080")
	viper.SetDefault(configPermitService, "permit-service:8080")
	viper.SetDefault(configSearchService, "search-service:8080")
	viper.SetDefault(configSpikeService, "spike-service:8080")
	viper.SetDefault(configGotenbergService, "gotenberg-service:8080")
	viper.SetDefault(configSecret, "pandora@drive")
	viper.SetDefault(configAuthURL, "http://localhost/auth/login")
	viper.SetDefault(configDocsURL, "http://localhost:3000")
	viper.SetDefault(configExternalApmURL, "http://localhost:8200")
	viper.SetDefault(configAllowOrigins, "http://localhost*")
	viper.SetDefault(configSupportLink, "https://open.rocket.chat")
	viper.SetDefault(configDropboxSupportLink, "https://open.rocket.chat")
	viper.SetDefault(configElasticsearchURL, "http://localhost:9200")
	viper.SetDefault(configElasticsearchUser, "")
	viper.SetDefault(configElasticsearchPassword, "")
	viper.SetDefault(configElasticsearchIndex, "metrics")
	viper.SetDefault(configTLSSkipVerify, true)
	viper.SetDefault(configElasticsearchSniff, false)
	viper.SetDefault(configHealthCheckInterval, 5)
	viper.SetDefault(configHealthCheckRPCTimeout, 5)
	viper.SetDefault(configApprovalServiceURL, "http://approval.service")
	viper.SetDefault(configExternalShareName, "שיתוף חיצוני")
	viper.SetDefault(configMyExternalSharesName, "השיתופים החיצוניים שלי")
	viper.SetDefault(configVipService, "http://localhost:8094")
	viper.SetDefault(configEnableExternalShare, false)
	viper.SetDefault(configWhiteListText, "או להיות מאושר באופן מיוחד")
	viper.SetDefault(user.ConfigBucketPostfix, "")
	viper.SetDefault(auth.ConfigWebUI, "http://localhost")
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
