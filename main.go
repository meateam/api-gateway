package main

import (
	"net/http"
	"time"

	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("port", 8080)
	viper.SetEnvPrefix("GW")
	viper.AutomaticEnv()
}

func main() {
	router := setupRouter()
	port := viper.GetString("port")
	s := &http.Server{
		Addr:           ":" + port,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	s.ListenAndServe()
}
