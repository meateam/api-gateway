module github.com/meateam/api-gateway

go 1.13

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v0.0.0-20190424000812-bd1331c62cae
	github.com/gin-gonic/gin v1.4.0
	github.com/grpc-ecosystem/grpc-gateway v1.11.1
	github.com/meateam/download-service v0.0.0-20190707094647-f4db0fc5fdaa
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20190829065145-d4d5344e0b43
	github.com/meateam/permission-service v0.0.0-20191029101002-980dd2c31d08
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/meateam/user-service v0.0.0-20191023124015-0d5945941f83
	github.com/processout/grpc-go-pool v1.2.1
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.4.0
	go.elastic.co/apm v1.5.0
	go.elastic.co/apm/module/apmgin v1.5.0
	go.elastic.co/apm/module/apmgrpc v1.5.0
	go.elastic.co/apm/module/apmhttp v1.5.0
	golang.org/x/net v0.0.0-20191101175033-0deb6923b6d9 // indirect
	golang.org/x/sys v0.0.0-20191104094858-e8c54fb511f6 // indirect
	google.golang.org/genproto v0.0.0-20191028173616-919d9bdd9fe6 // indirect
	google.golang.org/grpc v1.24.0
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
