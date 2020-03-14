module github.com/meateam/api-gateway

go 1.14

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.5.0
	github.com/grpc-ecosystem/grpc-gateway v1.14.3
	github.com/klauspost/compress v1.10.3
	github.com/meateam/download-service v0.0.0-20191216103739-80620a5c7311
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20200105125137-6b3c21f2bcf6
	github.com/meateam/gotenberg-go-client/v6 v6.0.7
	github.com/meateam/permission-service v0.0.0-20200105125125-65bb1b8801f1
	github.com/meateam/search-service v0.0.0-20191216105153-4b8090ce4489
	github.com/meateam/upload-service v0.0.0-20191216095848-6216c791f102
	github.com/meateam/user-service v0.0.0-20200105125121-7ce90f7c2081
	github.com/minio/minio v0.0.0-20200314070115-10fd53d6bbc8
	github.com/olivere/elastic/v7 v7.0.12
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.6.2
	go.elastic.co/apm v1.7.1
	go.elastic.co/apm/module/apmgin v1.7.1
	go.elastic.co/apm/module/apmgrpc v1.7.1
	go.elastic.co/apm/module/apmhttp v1.7.1
	google.golang.org/grpc v1.28.0
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota
