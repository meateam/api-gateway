module github.com/meateam/api-gateway

go 1.12

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v0.0.0-20190424000812-bd1331c62cae
	github.com/gin-gonic/gin v1.3.0
	github.com/grpc-ecosystem/grpc-gateway v1.8.6
	github.com/meateam/download-service v0.0.0-20190505082208-15b980fc9a07
	github.com/meateam/elasticsearch-logger v1.1.2
	github.com/meateam/file-service v0.0.0-20190617072641-600b1c4d91d3
	github.com/meateam/upload-service v0.0.0-20190505081218-33fd5544ae26
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.3.2
	go.elastic.co/apm v1.3.0
	go.elastic.co/apm/module/apmgin v1.3.0
	go.elastic.co/apm/module/apmgrpc v1.3.0
	go.elastic.co/apm/module/apmhttp v1.3.0
	google.golang.org/grpc v1.21.0
)

replace github.com/meateam/api-gateway/logger => ./logger
