package main

import (
	"net/http"

	ilogger "github.com/meateam/elasticsearch-logger"
	"github.com/spf13/viper"
)

const (
	envPrefix             = "GW"
	configPort            = "port"
	configUploadService   = "upload_service"
	configDownloadService = "download_service"
	configfileService     = "file_service"
)

var (
	logger = ilogger.NewLogger()
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetDefault(configDownloadService, "download-service:8080")
	viper.SetDefault(configfileService, "file-service:8080")
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

func main() {
	router, close := setupRouter()
	defer close()
	s := &http.Server{
		Addr:           ":" + viper.GetString(configPort),
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
	}

	logger.Infof("server listening on port: %s", viper.GetString(configPort))
	if err := s.ListenAndServe(); err != nil {
		logger.Fatalf("%v", err)
	}
}
