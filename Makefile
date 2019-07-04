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
