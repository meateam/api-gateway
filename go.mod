module github.com/meateam/api-gateway

go 1.13

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-gonic/gin v1.5.0
	github.com/go-openapi/runtime v0.19.23
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1
	github.com/meateam/delegation-service v0.0.0-20191218064947-abf0a5785bdc
	github.com/meateam/download-service v0.0.0-20191216103739-80620a5c7311
	github.com/meateam/dropbox-service v0.0.0-20210214104618-38ae427e485a
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20201029090524-223240db6f1e
	github.com/meateam/gotenberg-go-client/v6 v6.0.7
	github.com/meateam/grpc-go-conn-pool v0.0.0-20201214083317-16d5ec9ea3b8
	github.com/meateam/permission-service v0.0.0-20201227160413-b8b9c077c53d
	github.com/meateam/permit-service v0.0.0-20200205134633-2b7d9a5433c9
	github.com/meateam/search-service v0.0.0-20191202135334-eca1d41057e0
	github.com/meateam/spike-service v0.0.0-20200707100230-2e9242b8e18a
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/meateam/user-service v2.1.1-0.20201224124158-ee5d834b0f10+incompatible
	github.com/olivere/elastic/v7 v7.0.22
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.6.1
	go.elastic.co/apm v1.6.0
	go.elastic.co/apm/module/apmgin v1.6.0
	go.elastic.co/apm/module/apmgrpc v1.6.0
	go.elastic.co/apm/module/apmhttp v1.6.0
	go.mongodb.org/mongo-driver v1.4.2
	golang.org/x/net v0.0.0-20201209123823-ac852fbbde11 // indirect
	golang.org/x/sys v0.0.0-20201211090839-8ad439b19e0f // indirect
	golang.org/x/text v0.3.4 // indirect
	google.golang.org/genproto v0.0.0-20201211151036-40ec1c210f7a // indirect
	google.golang.org/grpc v1.34.0
	google.golang.org/grpc/examples v0.0.0-20201212000604-81b95b1854d7 // indirect
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/meateam/api-gateway/factory => ./factory

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
