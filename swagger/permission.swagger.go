package swagger

import (
	"github.com/meateam/api-gateway/permission"
)

// swagger:route GET /files/{id}/permissions files getpermission
//
// File permissions
//
// This will returns the permissions to the file
//
// Schemes: http
// Responses:
// 	200: permissionsResponse

// swagger:parameters getpermission
type permissionsRequest struct {

	// The file id
	// required:true
	// in:query
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// An array of files
// swagger:response permissionsResponse
type permissionsResponse struct {

	// in:body
	Permissions []permission.Permission
}

// swagger:route PUT /files/{id}/permissions files putpermission
//
// Create permissions
//
// This will create the permissions to the file
//
// Schemes: http
// Responses:
// 	200: permissionResponse

// swagger:parameters putpermission
type putRequest struct {

	// The file id
	// required:true
	// in:query
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string

	// The request body for permission
	// in:body
	Details PermissionDetails
}

// PermissionDetails request body for create permission
type PermissionDetails struct {
	UserID   string `json:"userID"`
	Role     string `json:"role"`
	Override bool   `json:"override"`
}

// swagger:route DELETE /files/{id}/permissions files deletepermission
//
// Delete permissions
//
// This will delete the permissions from the file
//
// Schemes: http
// Responses:
// 	200: permissionResponse

// swagger:parameters deletepermission
type deleteRequest struct {

	// The file id
	// required:true
	// in:query
	ID string `json:"id"`

	// The user id
	// in:query
	UserID string `json:"userId"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// An array of files
// swagger:response permissionResponse
type permissionResponse struct {

	// in:body
	Permissions permission.Permission
}