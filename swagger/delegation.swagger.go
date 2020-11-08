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
// This will return a single external user by its id from the phonebook
//
// Schemes: http
// Responses:
// 	200: delegatorResponse

// swagger:parameters delegator
type delegatorRequest struct {

	// The delegator id
	// unique:true
	// required:true
	// in:path
	ID string `json:"id"`

	// The jwt key
	// example:Bearer &{jwt}
	// in:header
	Authorization string
}

// swagger:route GET /delegators delegation searchdelegator
//
// Search delegator
//
// This will return a single delegator by its partial name
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
	Authorization string
}

// An array of delegators returned
// swagger:response delegatorResponse
type delegatorResponse struct {
	// in:body
	Body dpb.GetUserResponse
}
