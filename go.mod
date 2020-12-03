module github.com/meateam/api-gateway

go 1.13

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-gonic/gin v1.5.0
	github.com/go-openapi/runtime v0.19.24
	github.com/go-openapi/validate v0.19.15 // indirect
	github.com/go-swagger/go-swagger v0.25.0 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1
	github.com/kr/pretty v0.2.1 // indirect
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/meateam/delegation-service v0.0.0-20191218064947-abf0a5785bdc
	github.com/meateam/download-service v0.0.0-20191216103739-80620a5c7311
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20201029090524-223240db6f1e
	github.com/meateam/gotenberg-go-client/v6 v6.0.7
	github.com/meateam/permission-service v0.0.0-20200204090700-2d1debc9dc9b
	github.com/meateam/permit-service v0.0.0-20200205134633-2b7d9a5433c9
	github.com/meateam/search-service v0.0.0-20191202135334-eca1d41057e0
	github.com/meateam/spike-service v0.0.0-20200707100230-2e9242b8e18a
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/meateam/user-service v0.0.0-20200727105634-935fd55aab5e
	github.com/olivere/elastic/v7 v7.0.9
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/afero v1.4.1 // indirect
	github.com/spf13/viper v1.7.1
	go.elastic.co/apm v1.6.0
	go.elastic.co/apm/module/apmgin v1.6.0
	go.elastic.co/apm/module/apmgrpc v1.6.0
	go.elastic.co/apm/module/apmhttp v1.6.0
	go.mongodb.org/mongo-driver v1.4.4
	golang.org/x/mod v0.4.0 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb // indirect
	golang.org/x/sys v0.0.0-20201202213521-69691e467435 // indirect
	golang.org/x/tools v0.0.0-20201202200335-bef1c476418a // indirect
	google.golang.org/grpc v1.27.0
	gopkg.in/ini.v1 v1.62.0 // indirect
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
