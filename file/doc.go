/*
Package file is used to handle all file-service and download-service related operations.
Handling is implemented with a HTTP router returned from NewRouter and setup its routes
using Setup.
*/
package file

import (
	fpb "github.com/meateam/file-service/proto/file"
)

// swagger:response filesResponse
type filesResponse struct {
	// in:body
	Body []fpb.File
}
