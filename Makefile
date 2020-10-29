# Basic go commands
PROTOC=protoc

# Binary names
BINARY_NAME=api-gateway

all: clean fmt test build
build:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) -v
test:
		go test -v ./...
clean:
		go clean
		sudo rm -rf $(BINARY_NAME)
run: build
		./$(BINARY_NAME)
fmt:
		./gofmt.sh

#alias swagger="docker run --rm -it -e GOPATH=$HOME/go:/go -v $HOME:$HOME -w $(pwd) quay.io/goswagger/swagger"
# Swagger
install_swagger:
	which swagger || go get -u github.com/go-swagger/go-swagger/cmd/swagger

swagger: install_swagger
	swagger generate spec -o ./swagger.json --scan-models

