package main

import (
	"log"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

const (
	envPrefix           = "GW"
	configPort          = "port"
	configUploadService = "upload_service"
)

func init() {
	viper.SetDefault(configPort, 8080)
	viper.SetDefault(configUploadService, "upload-service:8080")
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

func main() {
	router, close := setupRouter()
	defer close()
	s := &http.Server{
		Addr:           ":" + viper.GetString(configPort),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("%v", err)
	}
}
