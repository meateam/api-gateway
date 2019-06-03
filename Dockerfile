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
FROM scratch
LABEL Name=api-gateway Version=0.0.1
COPY --from=builder /go/src/app/api-gateway /api-gateway
EXPOSE 8080
ENTRYPOINT ["/api-gateway"]
