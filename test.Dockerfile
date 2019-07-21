FROM golang:alpine
LABEL description="test"
RUN apk add --no-cache git
ENV GO111MODULE=on
ENV CGO_ENABLED=0
WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENTRYPOINT ["go", "test", "-v", "./..."]