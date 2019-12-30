module github.com/meateam/api-gateway

go 1.13

require (
	github.com/aws/aws-sdk-go v1.26.4 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/elastic/go-sysinfo v1.2.0 // indirect
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-gonic/gin v1.5.0
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1
	github.com/json-iterator/go v1.1.8 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/mattn/go-isatty v0.0.11 // indirect
	github.com/meateam/delegation-service v0.0.0-20191218064947-abf0a5785bdc
	github.com/meateam/download-service v0.0.0-20191216103739-80620a5c7311
	github.com/meateam/elasticsearch-logger v1.1.3-0.20190901111807-4e8b84fb9fda
	github.com/meateam/file-service/proto v0.0.0-20191216112842-3e40c18a428f
	github.com/meateam/gotenberg-go-client/v6 v6.0.6
	github.com/meateam/permission-service v0.0.0-20191216125530-dd8198aebe47
	github.com/meateam/permit-service v0.0.0-20191229091857-94950cdd1c2e
	github.com/meateam/search-service v0.0.0-20191216105153-4b8090ce4489
	github.com/meateam/spike-service v0.0.0-20191218082801-258e86a00bce
	github.com/meateam/upload-service v0.0.0-20191216095848-6216c791f102
	github.com/meateam/user-service v0.0.0-20191216110641-de4c18896763
	github.com/olivere/elastic/v7 v7.0.9
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/prometheus/procfs v0.0.8 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.6.1
	go.elastic.co/apm v1.6.0
	go.elastic.co/apm/module/apmgin v1.6.0
	go.elastic.co/apm/module/apmgrpc v1.6.0
	go.elastic.co/apm/module/apmhttp v1.6.0
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 // indirect
	golang.org/x/sys v0.0.0-20191228213918-04cbcbbfeed8 // indirect
	google.golang.org/genproto v0.0.0-20191223191004-3caeed10a8bf // indirect
	google.golang.org/grpc v1.26.0
	gopkg.in/go-playground/validator.v9 v9.30.2 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
)

replace github.com/meateam/api-gateway/logger => ./logger

replace github.com/meateam/api-gateway/user => ./user

replace github.com/meateam/api-gateway/upload => ./upload

replace github.com/meateam/api-gateway/server => ./server

replace github.com/meateam/api-gateway/file => ./file

replace github.com/meateam/api-gateway/quota => ./quota

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
