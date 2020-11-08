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
	Authorization string
}

// An array of files
// swagger:response permissionResponse
type permissionResponse struct {

	// in:body
	Permissions permission.Permission
}