/***
 *       __   _   _
 *      / _| (_) | |
 *     | |_   _  | |   ___   ___
 *     |  _| | | | |  / _ \ / __|
 *     | |   | | | | |  __/ \__ \
 *     |_|   |_| |_|  \___| |___/
 *
 *
 */
package swagger

import (
	file "github.com/meateam/api-gateway/file"
)

// swagger:route GET /files files listFiles
//
// List of files
//
// This returns all files according to the requested folder
//
// Schemes: http
// Responses:
// 	200: filesResponse

// swagger:parameters listFiles
type filesRequest struct {
	// The parent of the files
	// unique:true
	// in:url
	Parent string

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	Authorization string
}

// swagger:route GET /files/{id} files file
//
// Single file
//
// This returns a single file according to the requested folder
//
// Schemes: http
// Responses:
// 	200: fileResponse

// swagger:parameters file
type fileRequest struct {
	// The file id
	// unique:true
	// required:true
	// in:path
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// swagger:route GET /files/{id}?alt=media files downloadFile
//
// Single file content
//
// This download a single file according to the requested folder
//
// Schemes: http
// Responses:
// 	200: downloadFileResponse

// swagger:parameters downloadFile
type downloadFileRequest struct {
	// The file id
	// unique:true
	// required:true
	// in:path
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// An array of files
// swagger:response filesResponse
type filesResponse struct {
	// in:body
	Body []file.GetFileByIDResponse
}

// Single file
// swagger:response fileResponse
type fileResponse struct {
	// in:body
	Body file.GetFileByIDResponse
}

// Single file content
// swagger:response downloadFileResponse
type downloadFileResponse struct {
	
}

// swagger:route GET /files/{id}/ancestors files fileAncestors
//
// Returns the file's ancestors
//
// This returns all of the ancestors of a given file
//
// Schemes: http
// Responses:
// 	200: fileAncestorsResponse

// swagger:parameters fileAncestors
type fileAncestorsRequest struct {
	// The file id
	// unique:true
	// in:path
	// required:true
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// File ancestors
// swagger:response fileAncestorsResponse
type fileAncestorsResponse struct {
	// in:body
	Ancestors []string
}

// swagger:route DELETE /files/{id} files deleteFile
//
// Delete file
//
// This deletes the file according to its id
//
// Schemes: http
// responses:
//	200: DeleteResponse

// swagger:parameters deleteFile
type deleteFileRequest struct {
	// The file id to delete
	// unique:true
	// in:path
	// required:true
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// An array of ids
// swagger:response DeleteResponse
type DeleteResponse struct {
	// in:body
	ID []string
}

// swagger:route PUT /files/{id} files updateFile
//
// Update file
//
// This updates the file according to its id
//
// Schemes: http
// responses:
//	200: UpdateResponse

// swagger:parameters updateFile
type updateRequest struct {
	// The file id to update
	// unique:true
	// in:path
	// required:true
	ID string `json:"id"`

	// The update info
	// in:body
	PartialFile file.GetFileByIDResponse

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// swagger:route PUT /files files updateFiles
//
// Update files
//
// Updates many files with the same value.
//
// Schemes: http
// responses:
//	200: UpdateResponse

// swagger:parameters updateFiles
type updateFileRequest struct {
	// Array of ids to update.
	// in:body
	// required:true
	IDs []string

	// The update info
	// in:body
	// required:true
	PartialFile file.GetFileByIDResponse

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// An array of ids Of failed files
// swagger:response UpdateResponse
type UpdateResponse struct {
	// in:body
	FailedFilesID []string
}
