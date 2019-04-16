# api-gateway

api gateway

## Perform a simple media upload

`curl -X POST http://localhost:8080/upload?uploadType=media --data-binary "@/path/to/file" -H "Content-Type: <file_mime_type>" -H "Content-Disposition: filename=<file_name>"`

## Perform a simple multipart upload

`curl -X POST http://localhost:8080/upload?uploadType=multipart -H "Content-Type: multipart/form-data" -F "file=@/path/to/file"`
