package server

import (
	"net/http"

	"github.com/meateam/api-gateway/server/auth"
	"github.com/meateam/api-gateway/user"
	ilogger "github.com/meateam/elasticsearch-logger"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	"github.com/spf13/viper"
)

const (	
	envPrefix                   			= "GW"
	configPort                  			= "port"
	configUploadService         			= "upload_service"
	configDocsSecret            			= "docs_secret"
	configDownloadService       			= "download_service"
	configFileService           			= "file_service"
	configUserService           			= "user_service"
	configPermissionService     			= "permission_service"
	configDropboxService        			= "dropbox_service"
	configSearchService         			= "search_service"
	configSpikeService          			= "spike_service"
	configGotenbergService      			= "gotenberg_service"
	configSecret                			= "secret"
	configAuthURL               			= "auth_url"
	configDocsURL               			= "docs_url"
	configExternalApmURL        			= "external_apm_url"
	configAllowOrigins          			= "allow_origins"
	configSupportLink           			= "support_link"
	configDropboxSupportLink    			= "dropbox_support_link"
	configDownloadChromeURL     			= "chrome_download_url"
	configElasticsearchURL      			= "elasticsearch_url"
	configElasticsearchUser     			= "elasticsearch_user"
	configElasticsearchPassword 			= "elasticsearch_password"
	configElasticsearchIndex    			= "elasticsearch_index"
	configTLSSkipVerify         			= "tls_skip_verify"
	configElasticsearchSniff    			= "elasticsearch_sniff"
	configHealthCheckInterval   			= "health_check_interval"
	configHealthCheckRPCTimeout 			= "health_check_rpc_timeout"
	configApprovalServiceURL    			= "approval_url"
	configApprovalServiceUIURL  			= "approval_ui_url"
	configApprovalCtsServiceURL 			= "approval_cts_url"
	configApprovalCtsServiceUIURL  			= "approval_cts_ui_url"
	configTomcalDestName					= "tomcal_dest_name"
	configTomcalDestValue					= "tomcal_dest_value"
	configTomcalDestAppID					= "tomcal_dest_appid"
	configCtsDestName						= "cts_dest_name"
	configCtsDestValue						= "cts_dest_value"
	configCtsDestAppID						= "cts_dest_appid"
	configTransferStatusSuccess				= "transfer_status_success_type"
	configTransferStatusFailed				= "transfer_status_failed_type"
	configTransferStatusInProgress			= "transfer_status_in_progress_type"
	configExternalShareName     			= "external_share_name"
	configMyExternalSharesName  			= "my_external_shares_name"
	configVipService            			= "vip_service"
	configEnableExternalShare   			= "enable_external_share"
	configWhiteListText         			= "white_list_text"
	configBereshitSupportLink   			= "bereshit_support_link"
	configBamSupportNumber      			= "bam_support_number"
	configSwaggerPathFile       			= "swagger_path_file"
	configShowSwaggerUI         			= "show_swagger_ui"
	configPoolSize              			= "pool_size"
)

var (
	logger = ilogger.NewLogger()
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetDefault(configDocsSecret, "docs@drive")
	viper.SetDefault(configDownloadService, "download-service:8080")
	viper.SetDefault(configFileService, "file-service:8080")
	viper.SetDefault(configUserService, "user-service:8080")
	viper.SetDefault(configPermissionService, "permission-service:8080")
	viper.SetDefault(configDropboxService, "dropbox-service:8080")
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
	viper.SetDefault(configApprovalServiceUIURL, "http://approval.service.ui")
	viper.SetDefault(configApprovalCtsServiceURL, "http://approval.service")
	viper.SetDefault(configApprovalCtsServiceUIURL, "http://approval.service.ui")
	viper.SetDefault(configTomcalDestName, "תומכל")
	viper.SetDefault(configTomcalDestValue, "TOMCAL")
	viper.SetDefault(configTomcalDestAppID, "dropbox")
	viper.SetDefault(configCtsDestName, "CTS")
	viper.SetDefault(configCtsDestValue, "CTS")
	viper.SetDefault(configCtsDestAppID, "cargo")
	viper.SetDefault(configExternalShareName, "שיתוף חיצוני")
	viper.SetDefault(configMyExternalSharesName, "השיתופים החיצוניים שלי")
	viper.SetDefault(configVipService, "http://localhost:8094")
	viper.SetDefault(configShowSwaggerUI, false)
	viper.SetDefault(configEnableExternalShare, false)
	viper.SetDefault(configWhiteListText, "או להיות מאושר באופן מיוחד")
	viper.SetDefault(configBereshitSupportLink, "https://open.rocket.chat")
	viper.SetDefault(configBamSupportNumber, "03555555")
	viper.SetDefault(configSwaggerPathFile, "./swagger/ui")
	viper.SetDefault(user.ConfigBucketPostfix, "")
	viper.SetDefault(auth.ConfigWebUI, "http://localhost")
	viper.SetDefault(configPoolSize, 4)
	viper.SetDefault(configTransferStatusSuccess, "success")
	viper.SetDefault(configTransferStatusFailed, "failed")
	viper.SetDefault(configTransferStatusInProgress, "in progress")
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

// Server is a structure that holds the http server of the api-gateway.
type Server struct {
	server *http.Server
	conns  []*grpcPoolTypes.ConnPool
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
			(*v).Close()
		}
	}()

	logger.Infof("server listening on port: %s", viper.GetString(configPort))
	if err := s.server.ListenAndServe(); err != nil {
		logger.Fatalf("%v", err)
	}
}
