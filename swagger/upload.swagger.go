/***
 *                      _                       _
 *                     | |                     | |
 *      _   _   _ __   | |   ___     __ _    __| |
 *     | | | | | '_ \  | |  / _ \   / _` |  / _` |
 *     | |_| | | |_) | | | | (_) | | (_| | | (_| |
 *      \__,_| | .__/  |_|  \___/   \__,_|  \__,_|
 *             | |
 *             |_|
 */

package swagger

import (
	"bytes"
)

// swagger:route POST /upload upload uploadfolder
//
// Upload folder
//
// Uploads a folder.
//
// Schemes: http
// responses:
//	200: UploadResponse

// swagger:parameters uploadfolder
type uploadFolderRequest struct {
	// The parent of the new folder
	// in:query
	Parent string

	// Folder type.
	// example:application/vnd.drive.folder
	// in:header
	ContentType string `json:"Content-Type"`

	// The folder name
	// example:filename=folderName
	// in:header
	ContentDisposition string `json:"Content-Disposition"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	Authorization string
}

// swagger:route POST /upload?UploadType=multipart upload uploadmultipart
//
// Upload multipart
//
// Upload a small file under 5MB .
//
// Schemes: http
// responses:
//	200: UploadResponse

// swagger:parameters uploadmultipart
type uploadMultipartRequest struct {
	// Upload type.
	// example:multipart
	// in:query
	UploadType string

	// The parent of the new file
	// in:query
	Parent string

	// The new file metadata
	// in:formData
	// swagger:file
	File *bytes.Buffer `json:"file"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header

	Authorization string
}

// swagger:route POST /upload?UploadType= upload initresumable
//
// Init resumable upload
//
// Initializes the resumable upload.
//
// Schemes: http
// responses:
//	200: initResumableResponse

// swagger:parameters initresumable
type InitResumableRequest struct {
	// The parent of the new file
	// in:query
	Parent string

	// The file size
	// in:header
	XContentLength string `json:"X-Content-Length"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header

	Authorization string

	// The request body for upload resumable
	// in:body
	Details FileDetails
}

// FileDetails request body for upload resumable
type FileDetails struct {
	MimeType string `json:"mimeType"`
	Title    string `json:"title"`
}

// Upload id
// swagger:response initResumableResponse
type InitResumableResponse struct {
	// in:header
	UploadID string `json:"X-Uploadid"`
}

// swagger:route POST /upload?UploadType=resumable upload uploadresumable
//
// Upload resumable
//
// Upload a big file over 5MB and up to 5TB .
// Runs after the init resumable upload
//
// Schemes: http
// responses:
//	200: UploadResponse

// swagger:parameters uploadresumable
type uploadResumableRequest struct {
	// Upload type.
	// example:resumable
	// in:query
	UploadType string

	// Upload id from init resumable upload.
	// example:5e23e4a5-027a-431b-bd67-39e46b59595a
	// in:query
	UploadID string `json:"uploadId"`

	// The parent of the new file
	// in:query
	Parent string

	// The new file
	// in:formData
	// swagger:file
	File *bytes.Buffer `json:"file"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header

	Authorization string
}

// swagger:route PUT /upload/{id} upload updateFileContent
//
// Update file content
//
// This will Update the contents of the file according to its ID
//
// Schemes: http
// responses:
//	200: UploadResponse

// swagger:parameters updateFileContent
type updateContentRequest struct {
	// The file id to update
	// unique:true
	// in:path
	ID string `json:"id"`

	// The new file content
	// in:formData
	// swagger:file
	File *bytes.Buffer `json:"file"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	Authorization string
}

// File id
// swagger:response UploadResponse
type UploadResponse struct {
	// in:body
	ID string
}
