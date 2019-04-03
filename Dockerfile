#build stage
FROM golang:alpine AS builder
ENV GO111MODULE=on
WORKDIR /go/src/app
RUN apk add --no-cache git make
COPY . .
RUN make build

#final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/app/api-gateway /api-gateway
ENTRYPOINT ./api-gateway
LABEL Name=api-gateway Version=0.0.1
EXPOSE 8080
