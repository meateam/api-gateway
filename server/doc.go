/*
Package server is the api-gateway server, handling all its requests with a HTTP server.
Call NewServer to get a new server.Server instance, and Use Server.Listen to listen and
serve the server on a port.
Configuration is done using environment variables:
GW_PORT: Port on which the server would listen.
	default: 8080
GW_AUTH_URL: The url of the authentication service to redirect the user to authenticate.
	default: "http://localhost/auth/login"
GW_DOCS_URL: The url of the edit docs.
	default: "http://localhost/3000/api/files"
GW_SECRET: The secret used to validate authorization jwt tokens.
	default: "pandora@drive"
GW_UPLOAD_SERVICE: The address of the upload-service.
	default: "upload-service:8080"
GW_DOWNLOAD_SERVICE: The address of the download-service.
	default: "download-service:8080"
GW_FILE_SERVICE: The address of the file-service.
	default: "file-service:8080"
GW_EXTERNAL_APM_URL: External server's client APM url.
	default: "http://localhost:8200"
GW_ALLOW_ORIGINS: List of allowed origins for the Access-Control-Allow-Origin header.
	default: "http://localhost*", can be a list of urls.

For configuring the APM agent see https://www.elastic.co/guide/en/apm/agent/go/current/configuration.html
For configuring the logger of the server see Package logger doc.go
*/
package server
