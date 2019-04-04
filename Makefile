# Basic go commands
PROTOC=protoc

# Binary names
BINARY_NAME=api-gateway

all: clean test build
build:
		go build -o $(BINARY_NAME) -v
test:
		go test -v ./...
clean:
		go clean
		sudo rm -rf $(BINARY_NAME)
run:
		go build -o $(BINARY_NAME) -v
		./$(BINARY_NAME)
