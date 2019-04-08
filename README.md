# api-gateway
api gateway

## Perform a simple upload
`curl -X POST http://localhost:8080/upload?uploadType=media --data-binary "@/path/to/file" -H "Content-Type: <file_mime_type>"`