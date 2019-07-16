# api-gateway
[![Go Report Card](https://goreportcard.com/badge/github.com/meateam/api-gateway)](https://goreportcard.com/report/github.com/meateam/api-gateway)
[![GoDoc](https://godoc.org/github.com/meateam/api-gateway?status.svg)](https://godoc.org/github.com/meateam/api-gateway)
## Perform a simple media upload

`curl -X POST http://localhost:8080/api/upload?uploadType=media --data-binary "@/path/to/file" -H "Authorization: Bearer <jwt_token>" -H "Content-Type: <file_mime_type>" -H "Content-Disposition: filename=<file_name>"`

## Perform a simple multipart upload

`curl -X POST http://localhost:8080/api/upload?uploadType=multipart -H "Authorization: Bearer <jwt_token>" -H "Content-Type: multipart/form-data" -F "file=@/path/to/file"`
