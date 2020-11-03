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
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// The quota object
// swagger:response getSearchResponse
type getSearchResponse struct {

	// in:body
	Files []file.GetFileByIDResponse
}