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
// This will return all files according to the requested folder
//
// Schemes: http
// Responses:
// 	200: filesResponse

// swagger:parameters listFiles
type filesRequest struct {
	// The file parent
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
// This will returns a single file by its id
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

// swagger:route GET /files/{id}/ancestors files fileAncestors
//
// Returns a file ancestors
//
// This will returns all file ancestors
//
// Schemes: http
// Responses:
// 	200: fileAncestorsResponse

// swagger:parameters fileAncestors
type fileAncestorsRequest struct {
	// The file id to get is ancestors
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
// This will deletes the file according to its ID
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
// This will update the file according to its ID
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

	// The partial file to update
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

	// The partial file to update
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
