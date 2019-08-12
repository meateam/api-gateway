package test

import (
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const gwSecretKey = "GW_SECRET"

var jwtKey = os.Getenv(gwSecretKey)

// GenerateJwtToken generates a jwt token for testing purposes
func GenerateJwtToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.MapClaims{
		"id":        "5cb72ad5b06cc14394c1d632",
		"firstName": "Elad",
		"lastName":  "Biran",
		"mail":      "elad@rabiran",
		"iat":       time.Now().Unix(),
		"exp":       time.Now().AddDate(0, 1, 0).Unix(),
	})

	return token.SignedString([]byte(jwtKey))
}
