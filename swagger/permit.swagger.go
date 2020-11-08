package swagger

import (
	ppb "github.com/meateam/permit-service/proto"
)

// swagger:route GET /files/{id}/permits files getpermits
//
// File permits
//
// This returns the permits of a given file
//
// Schemes: http
// Responses:
// 	200: permitsResponse

// swagger:parameters getpermits
type permitsRequest struct {

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

// An array of permits
// swagger:response permitsResponse
type permitsResponse struct {

	// in:body
	Permits []ppb.UserStatus
}

// swagger:route PUT /files/{id}/permits files putpermits
//
// Create file permits
//
// This creates the permits of a given file
//
// Schemes: http
// Responses:
// 	200: permitResponse

// swagger:parameters putpermits
type putPremitRequest struct {

	// The file id
	// required:true
	// in:query
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string

	// The request body for creating a permit
	// in:body
	Details PermitDetails
}

// PermitDetails request body for creating a permit
type PermitDetails struct {
	FileName       string   `json:"fileName"`
	Users          []User   `json:"users,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Info           string   `json:"info,omitempty"`
	Approvers      []string `json:"approvers,omitempty"`
}

// User details
type User struct {
	ID       string `json:"id,omitempty"`
	FullName string `json:"full_name,omitempty"`
}

// An array of permits
// swagger:response permitResponse
type permitResponse struct {

	// in:body
	Permits ppb.CreatePermitResponse
}

// rg.PATCH(fmt.Sprintf("/permits/:%s", ParamReqID), checkStatusScope, r.UpdateStatus)

// swagger:route PATCH /files/{id}/permits files patchpermits
//
// Update file permits
//
// This will update the permits to the file
//
// Schemes: http
// Responses:
// 	200:

// swagger:parameters patchpermits
type patchPremitRequest struct {

	// The file id
	// required:true
	// in:query
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string

	// The request body for update permit
	// in:body
	Details UpdatePermitDetails
}

// UpdatePermitDetails request body for updating a permit
type UpdatePermitDetails struct {
	Status string `json:"status,omitempty"`
}
