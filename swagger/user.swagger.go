package swagger

import (
	upb "github.com/meateam/user-service/proto/users"
)

// swagger:route GET /users/{id} users getuser
//
// Get user
//
// This will returns a single user by its id
//
// Schemes: http
// Responses:
// 	200: userResponse

// swagger:parameters getUser
type getUserRequest struct {

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

// swagger:route GET /users users searchUserRequest
//
// Search user
//
// This will search user by its name
//
// Schemes: http
// Responses:
// 	200: userResponse

// swagger:parameters searchUserRequest
type searchUserRequest struct {

	// The user name
	// required:true
	// in:query
	Partial string `json:"partial"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// The user object
// swagger:response userResponse
type getUserResponse struct {

	// in:body
	User upb.GetUserResponse
}

// rg.GET(fmt.Sprintf("/users/:%s/approverInfo", ParamUserID), r.GetApproverInfo)

// swagger:route GET /users/{id}/approverInfo users approverinfo
//
// User approver info
//
// This will returns the approver info of user
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
	User upb.GetApproverInfoResponse
}
