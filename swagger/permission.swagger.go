package swagger

import (
	"github.com/meateam/api-gateway/permission"
)

// swagger:route GET /files/{id}/permissions files getpermission
//
// File permissions
//
// This returns the permissions of a given file
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
// This creates a permissions to a given file
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
	Authorization string

	// The request body for permission
	// in:body
	Details PermissionDetails
}

// PermissionDetails request body for creating permission
type PermissionDetails struct {
	UserID   string `json:"userID"`
	Role     string `json:"role"`
	Override bool   `json:"override"`
}

// swagger:route DELETE /files/{id}/permissions files deletepermission
//
// Delete permissions
//
// This deletes the permission of a file by its id and user
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
	Authorization string
}

// An array of permissions
// swagger:response permissionResponse
type permissionResponse struct {

	// in:body
	Permissions permission.Permission
}
