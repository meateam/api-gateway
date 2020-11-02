// Package classification Drive API.
//
// Terms Of Service:
//
//
//     Schemes: http
//     Host: localhost:8080
//     BasePath: /api
//     Version: v1.3
//     Contact: Drive team<drive.team@example.com> http://www.google.com
//
//     Consumes:
//     - application/json
//     - application/xml
//
//     Produces:
//     - application/json
//
// swagger:meta
package swagger

import (
	"bytes"

	file "github.com/meateam/api-gateway/file"
)

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
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// An array of files
// swagger:response filesResponse
type filesResponse struct {
	// in:body
	Body []file.GetFileByIDResponse
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
	// in:path
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
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
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
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
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
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
	ID string `json:"id"`

	// The partial file to update
	// in:body
	PartialFile file.GetFileByIDResponse

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
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
	IDs []string

	// The partial file to update
	// in:body
	PartialFile file.GetFileByIDResponse

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// An array of ids Of failed files
// swagger:response UpdateResponse
type UpdateResponse struct {
	// in:body
	FailedFilesID []string
}

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

// swagger:route POST /upload upload uploadfolder
//
// Upload folder
//
// Upload a folder.
//
// Schemes: http
// responses:
//	200: UploadResponse

// swagger:parameters uploadfolder
type uploadFolderRequest struct {
	// The parent folder to upload
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
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// swagger:route POST /upload/ upload uploadmultipart
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

	// The parent folder to upload
	// in:query
	Parent string

	// The folder name
	// in:formData
	// swagger:file
	File *bytes.Buffer `json:"file"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// File id
// swagger:response UploadResponse
type UploadResponse struct {
	// in:body
	ID string
}

//  rg.PUT("/upload/:"+ParamFileID, r.Update)
