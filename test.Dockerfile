FROM golang:alpine
LABEL description="test"
ENV GO111MODULE=on
ENV CGO_ENABLED=0
WORKDIR /go/src/app
RUN apk add --no-cache git
COPY . .
ENTRYPOINT ["go", "test", "-v", "./..."]