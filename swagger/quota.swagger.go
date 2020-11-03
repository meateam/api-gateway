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
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
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
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// The quota object
// swagger:response quotaResponse
type quotaResponse struct {

	// in:body
	Quota qpb.GetOwnerQuotaResponse
}