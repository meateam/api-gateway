package swagger

import(
	qpb "github.com/meateam/file-service/proto/quota"
)

// swagger:route GET /user/quota quota getquota
//
// My quota
//
// This will returns my current quota
//
// Schemes: http
// Responses:
// 	200: quotaResponse

// swagger:parameters getquota
type quotaRequest struct {

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// swagger:route GET /users/{id}/quota users getquotabyid
//
// User quota
//
// This will returns the quota of the user by its id
//
// Schemes: http
// Responses:
// 	200: quotaResponse

// swagger:parameters getquotabyid
type quotaByIDRequest struct {

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

// The quota object
// swagger:response quotaResponse
type quotaResponse struct {

	// in:body
	Quota qpb.GetOwnerQuotaResponse
}