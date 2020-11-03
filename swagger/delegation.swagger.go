/***
 *          _          _                          _     _                 
 *         | |        | |                        | |   (_)                
 *       __| |   ___  | |   ___    __ _    __ _  | |_   _    ___    _ __  
 *      / _` |  / _ \ | |  / _ \  / _` |  / _` | | __| | |  / _ \  | '_ \ 
 *     | (_| | |  __/ | | |  __/ | (_| | | (_| | | |_  | | | (_) | | | | |
 *      \__,_|  \___| |_|  \___|  \__, |  \__,_|  \__| |_|  \___/  |_| |_|
 *                                 __/ |                                  
 *                                |___/                                   
 */
package swagger

import (
	dpb "github.com/meateam/delegation-service/proto/delegation-service"
)

// swagger:route GET /delegators/{id} delegation delegator
//
// User
//
// This will returns a single user by its id
//
// Schemes: http
// Responses:
// 	200: delegatorResponse

// swagger:parameters delegator
type delegatorRequest struct {

	// The user id
	// unique:true
	// required:true
	// in:path
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// swagger:route GET /delegators delegation searchdelegator
//
// Search user
//
// This will returns a single user by its name
//
// Schemes: http
// Responses:
// 	200: delegatorResponse

// swagger:parameters searchdelegator
type searchRequest struct {

	// The user name
	// required:true
	// in:query
	Partial string `json:"partial"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	// default: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjVlNTY4ODMyNDIwM2ZjNDAwNDM1OTFhYSIsImFkZnNJZCI6InQyMzQ1ODc4OUBqZWxsby5jb20iLCJnZW5lc2lzSWQiOiI1ZTU2ODgzMjQyMDNmYzQwMDQzNTkxYWEiLCJuYW1lIjp7ImZpcnN0TmFtZSI6Iteg15nXmden15kiLCJsYXN0TmFtZSI6IteQ15PXmdeT16EifSwiZGlzcGxheU5hbWUiOiJ0MjM0NTg3ODlAamVsbG8uY29tIiwicHJvdmlkZXIiOiJHZW5lc2lzIiwiZW50aXR5VHlwZSI6ImRpZ2ltb24iLCJjdXJyZW50VW5pdCI6Im5pdHJvIHVuaXQiLCJkaXNjaGFyZ2VEYXkiOiIyMDIyLTExLTMwVDIyOjAwOjAwLjAwMFoiLCJyYW5rIjoibWVnYSIsImpvYiI6Iteo15XXpteXIiwicGhvbmVOdW1iZXJzIjpbIjAyNjY2Njk5OCIsIjA1Mi0xMjM0NTY3Il0sImFkZHJlc3MiOiLXqNeX15XXkSDXlNee157Xqten15nXnSAzNCIsInBob3RvIjpudWxsLCJqdGkiOiIyM2ZmYjFkOS1lYWMxLTRhNTItYWQyMC1jMTNmYzEyODM1MmMiLCJpYXQiOjE2MDQzNDgwNjIsImV4cCI6MTYwNjk0MDA2MiwiZmlyc3ROYW1lIjoi16DXmdeZ16fXmSIsImxhc3ROYW1lIjoi15DXk9eZ15PXoSJ9.bXSpUXJeKzCWwzOsDDVS0a8vjYAtQ406OogOxAmn8mM
	Authorization string
}

// An array of files
// swagger:response delegatorResponse
type delegatorResponse struct {
	// in:body
	Body dpb.GetUserResponse
}