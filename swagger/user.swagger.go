package swagger

import (
	upb "github.com/meateam/user-service/proto/users"
)

// swagger:route GET /users/{id} users getuser
//
// Get user
//
// This return a single user by its id
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
// This searches a user by its partial name
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
