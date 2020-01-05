module github.com/meateam/api-gateway

go 1.13

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v0.0.0-20190424000812-bd1331c62cae
	github.com/gin-gonic/gin v1.4.0
	github.com/grpc-ecosystem/grpc-gateway v1.11.1
	github.com/meateam/download-service v0.0.0-20191004151843-b4033e9b951c
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20191219073005-3114505d53bf
	github.com/meateam/gotenberg-go-client/v6 v6.0.7
	github.com/meateam/permission-service v0.0.0-20200101080310-b5f105d5c5fe
	github.com/meateam/search-service v0.0.0-20191202135334-eca1d41057e0
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/meateam/user-service v0.0.0-20191023124015-0d5945941f83
	github.com/olivere/elastic/v7 v7.0.9
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.6.1
	github.com/tevino/abool v0.0.0-20170917061928-9b9efcf221b5
	go.elastic.co/apm v1.6.0
	go.elastic.co/apm/module/apmgin v1.5.0
	go.elastic.co/apm/module/apmgrpc v1.6.0
	go.elastic.co/apm/module/apmhttp v1.6.0
	google.golang.org/grpc v1.26.0
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
