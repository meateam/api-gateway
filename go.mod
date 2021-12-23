module github.com/meateam/api-gateway

go 1.13

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-gonic/gin v1.7.1
	github.com/go-openapi/errors v0.20.0 // indirect
	github.com/go-openapi/runtime v0.19.29
	github.com/go-openapi/validate v0.20.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/meateam/download-service v0.0.0-20210214143255-2b7c813e14ef
	github.com/meateam/dropbox-service v0.0.0-20210323125524-40aa0b34499c
	github.com/meateam/elasticsearch-logger v1.2.0
	github.com/meateam/fav-service v0.0.0-20210714071938-14c0a1ce3904
	github.com/meateam/file-service/proto v0.0.0-20211209143512-0efc9e190036
	github.com/meateam/gotenberg-go-client/v6 v6.0.7
	github.com/meateam/grpc-go-conn-pool v0.0.0-20201214083317-16d5ec9ea3b8
	github.com/meateam/permission-service v0.0.0-20201227160413-b8b9c077c53d
	github.com/meateam/search-service v0.0.0-20191202135334-eca1d41057e0
	github.com/meateam/spike-service v0.0.0-20200707100230-2e9242b8e18a
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/meateam/user-service v3.1.1-0.20210518081330-33212ed397cb+incompatible
	github.com/olivere/elastic/v7 v7.0.22
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/afero v1.5.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1
	go.elastic.co/apm v1.7.0
	go.elastic.co/apm/module/apmgin v1.6.0
	go.elastic.co/apm/module/apmgrpc v1.7.0
	go.elastic.co/apm/module/apmhttp v1.7.0
	go.mongodb.org/mongo-driver v1.5.1
	google.golang.org/grpc v1.41.0
	google.golang.org/grpc/examples v0.0.0-20201212000604-81b95b1854d7 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/meateam/api-gateway/factory => ./factory

replace github.com/meateam/api-gateway/fav => ./fav

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
