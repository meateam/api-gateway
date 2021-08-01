#build stage
FROM golang:alpine AS builder
RUN apk add --no-cache git make
ENV GO111MODULE=on
WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

#final stage
FROM golang:alpine
RUN apk --no-cache add curl
LABEL Name=api-gateway Version=0.0.1
COPY --from=builder /go/src/app/api-gateway /api-gateway
COPY --from=builder /go/src/app/swagger/ /swagger/
EXPOSE 8080
ENTRYPOINT ["/api-gateway"]
