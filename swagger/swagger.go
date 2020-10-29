/***
 *       ▄████████  ▄█   ▄█          ▄████████    ▄████████ 
 *      ███    ███ ███  ███         ███    ███   ███    ███ 
 *      ███    █▀  ███▌ ███         ███    █▀    ███    █▀  
 *     ▄███▄▄▄     ███▌ ███        ▄███▄▄▄       ███        
 *    ▀▀███▀▀▀     ███▌ ███       ▀▀███▀▀▀     ▀███████████ 
 *      ███        ███  ███         ███    █▄           ███ 
 *      ███        ███  ███▌    ▄   ███    ███    ▄█    ███ 
 *      ███        █▀   █████▄▄██   ██████████  ▄████████▀  
 *                      ▀                                   
 */

// swagger:route GET /api/files files listFiles
//
// List of files
//
// This will return all files according to the requested folder
//
// Schemes: http
// Responses:
// 	200: filesResponse

// swagger:route GET /api/files/{id} files file
//
// File
//
// This will return a single file by its id
//
// Schemes: http
// Responses:
// 	200: fileResponse

// swagger:route DELETE /api/files/{id} files idParam
//
// Delete file
//
// This will remove file by its id
//
// Schemes: http
// responses:
//	200: idResponse