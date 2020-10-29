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

// swagger:response fileResponse
type fileResponse struct {
	// in:body
	Body GetFileByIDResponse
}



// swagger:response idResponse
type idResponse struct {
	// array of ids
	ID []string
}



// swagger:parameters idParam
type IDParam struct {
    // The id of the file
    // unique:true
	// in:url
	// required:true
    ID string `json:"id"`
}
