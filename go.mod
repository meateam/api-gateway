module github.com/meateam/api-gateway

go 1.13

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-contrib/static v0.0.0-20200916080430-d45d9a37d28e
	github.com/gin-gonic/gin v1.6.3
	github.com/go-delve/delve v1.5.0 // indirect
	github.com/go-openapi/analysis v0.19.11 // indirect
	github.com/go-openapi/errors v0.19.8 // indirect
	github.com/go-openapi/runtime v0.19.23
	github.com/go-openapi/spec v0.19.11 // indirect
	github.com/go-openapi/strfmt v0.19.7 // indirect
	github.com/go-openapi/swag v0.19.11 // indirect
	github.com/go-openapi/validate v0.19.12 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-dap v0.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/meateam/delegation-service v0.0.0-20191218064947-abf0a5785bdc
	github.com/meateam/download-service v0.0.0-20191216103739-80620a5c7311
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20201028140320-79a478fb5ba1
	github.com/meateam/gotenberg-go-client/v6 v6.0.7
	github.com/meateam/permission-service v0.0.0-20200204090700-2d1debc9dc9b
	github.com/meateam/permit-service v0.0.0-20200205134633-2b7d9a5433c9
	github.com/meateam/search-service v0.0.0-20191202135334-eca1d41057e0
	github.com/meateam/spike-service v0.0.0-20191218082801-258e86a00bce
	github.com/meateam/upload-service v0.0.0-20190829065259-6265a6168676
	github.com/meateam/user-service v0.0.0-20200727105634-935fd55aab5e
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/olivere/elastic/v7 v7.0.9
	github.com/peterh/liner v1.2.0 // indirect
	github.com/rakyll/statik v0.1.7 // indirect
	github.com/ribice/golang-swaggerui-example v0.0.0-20180611180427-1e7622a30e50
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1 // indirect
	github.com/spf13/viper v1.7.0
	github.com/swaggo/files v0.0.0-20190704085106-630677cd5c14
	github.com/swaggo/gin-swagger v1.3.0
	github.com/swaggo/swag v1.6.9
	github.com/urfave/cli v1.22.4 // indirect
	go.elastic.co/apm v1.6.0
	go.elastic.co/apm/module/apmgin v1.6.0
	go.elastic.co/apm/module/apmgrpc v1.6.0
	go.elastic.co/apm/module/apmhttp v1.6.0
	go.mongodb.org/mongo-driver v1.4.2
	go.starlark.net v0.0.0-20201014215153-dff0ae5b4820 // indirect
	golang.org/x/arch v0.0.0-20201008161808-52c3e6f60cff // indirect
	golang.org/x/net v0.0.0-20201027133719-8eef5233e2a1 // indirect
	golang.org/x/sys v0.0.0-20201028094953-708e7fb298ac // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/tools v0.0.0-20201028182000-5bbba6644ef5 // indirect
	google.golang.org/genproto v0.0.0-20201028140639-c77dae4b0522 // indirect
	google.golang.org/grpc v1.33.1
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/go-playground/validator.v9 v9.29.1 // indirect
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
