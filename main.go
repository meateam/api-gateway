package main

import (
	"log"
	"net/http"

	"github.com/spf13/viper"
)

const (
	envPrefix             = "GW"
	configPort            = "port"
	configUploadService   = "upload_service"
	configDownloadService = "download_service"
	configfileService     = "file_service"
	configSecret          = "secret"
	configHost            = "host"
	configAuthUrl         = "auth_url"
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetDefault(configDownloadService, "download-service:8080")
	viper.SetDefault(configfileService, "file-service:8080")
	viper.SetDefault(configSecret, "bLue5tream@2018")
	viper.SetDefault(configHost, "http://localhost")
	viper.SetDefault(configAuthUrl, "/auth/login")
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
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("%v", err)
	}
}
