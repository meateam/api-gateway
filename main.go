package main

import (
	"github.com/meateam/api-gateway/server"

	"github.com/spf13/viper"
)

const (
	envPrefix             = "GW"
	configPort            = "port"
	configUploadService   = "upload_service"
	configDownloadService = "download_service"
	configfileService     = "file_service"
	configSecret          = "secret"
	configAuthURL         = "auth_url"
	configExternalApmURL  = "external_apm_url"
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetDefault(configDownloadService, "download-service:8080")
	viper.SetDefault(configfileService, "file-service:8080")
	viper.SetDefault(configSecret, "pandora@drive")
	viper.SetDefault(configAuthURL, "http://localhost/auth/login")
	viper.SetDefault(configExternalApmURL, "http://localhost:8200")
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

func main() {
	server.NewServer().Listen()
}
