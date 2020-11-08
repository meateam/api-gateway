package swagger

import(
	file "github.com/meateam/api-gateway/file"
)

// swagger:route GET /search search getSearchRequest
//
// User quota by id
//
// This will returns the quota of the user by its id
//
// Schemes: http
// Responses:
// 	200: getSearchResponse

// swagger:parameters getSearchRequest
type getSearchRequest struct {

	// The search query
	// required:true
	// in:query
	Q string `json:"q"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// required:true
	Authorization string
}

// The quota object
// swagger:response getSearchResponse
type getSearchResponse struct {

	// in:body
	Files []file.GetFileByIDResponse
}