package swagger

import (
	drp "github.com/meateam/dropbox-service/proto/dropbox"
)

// rg.GET(fmt.Sprintf("/users/:%s/approverInfo", ParamUserID), r.GetApproverInfo)

// swagger:route GET /users/{id}/approverInfo users approverinfo
//
// User approver info
//
// This returns the approver info of a user by its id
//
// Schemes: http
// Responses:
// 	200: approverInfoResponse

// swagger:parameters approverinfo
type approverInfoRequest struct {

	// The user id
	// required:true
	// in:query
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// The approver info object
// swagger:response approverInfoResponse
type approverInfoResponse struct {

	// in:body
	User drp.GetApproverInfoResponse
}
