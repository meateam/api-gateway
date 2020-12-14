/*
	A guide to using swagger documentation

	Swagger and swagger ui documentation https://swagger.io/docs
	Swagger ui we will be used outside
	In swagger ui we can test the documentation

	Redoc git repo https://github.com/Redocly/redoc
	Redoc is a ui we using in inside net

	goswagger documentation https://goswagger.io
	go swagger runs on all the files under the swagger folder and creates swagger.json file

	How to use with goswagger
	all commands run in terminal
	install            -  go get -u github.com/go-swagger/go-swagger/cmd/swagger
	run on docker      -  alias swagger="docker run --rm -it -e GOPATH=$HOME/go:/go -v $HOME:$HOME -w $(pwd) quay.io/goswagger/swagger"
	create a json file -  swagger generate spec -o ./swagger/ui/swagger.json --scan-models
*/

// Package classification Drive API.
//
// Terms Of Service:
//
//
//     Schemes: http
//     Host: pandora.northeurope.cloudapp.azure.com
//     BasePath: /api
//     Version: v2.0.0
//     Contact: Drive team<drive.team@example.com> http://www.google.com
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// swagger:meta
package swagger
