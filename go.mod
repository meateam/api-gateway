module github.com/meateam/api-gateway

go 1.12

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v0.0.0-20190424000812-bd1331c62cae
	github.com/gin-gonic/gin v1.4.0
	github.com/grpc-ecosystem/grpc-gateway v1.8.6
	github.com/meateam/download-service v0.0.0-20190707094647-f4db0fc5fdaa
	github.com/meateam/elasticsearch-logger v1.1.2
	github.com/meateam/file-service/proto v0.0.0-20190829065145-d4d5344e0b43
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.3.2
	go.elastic.co/apm v1.3.0
	go.elastic.co/apm/module/apmgin v1.3.0
	go.elastic.co/apm/module/apmgrpc v1.3.0
	go.elastic.co/apm/module/apmhttp v1.3.0
	google.golang.org/grpc v1.23.0
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
