package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"

	oauth "github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/user"
	es "github.com/olivere/elastic/v7"
	"github.com/spf13/viper"
	"go.elastic.co/apm"
)

const (
	// NoAuthType states thate no auth type was declared in the headers
	NoAuthType = "NoneAuthType"

	// UnknownAppID is the app id given to unknown app ids
	UnknownAppID = "UnknownAppID"

	// ClientNameLabel is the claim name of the client name of the requesting external application
	ClientNameLabel = "clientName"
)

type extractedTokenInfo struct {
	appID    string
	authType string
}

type body struct {
	User      *user.User `json:"user,omitempty"`
	Path      string     `json:"path,omitempty"`
	Method    string     `json:"method,omitempty"`
	TimeStamp time.Time  `json:"timestamp,omitempty"`
	Date      time.Time  `json:"date,omitempty"`
	TraceID   string     `json:"traceID,omitempty"`
	AuthType  string     `json:"authType,omitempty"`
	AppID     string     `json:"appID,omitempty"`
}

// NewMetricsLogger initializes the metrics middleware.
func NewMetricsLogger() gin.HandlerFunc {
	config, index := initESConfig()

	client, err := es.NewClient(config...)
	if err != nil {
		return nil
	}

	return func(c *gin.Context) {

		currentTransaction := apm.TransactionFromContext(c.Request.Context())

		reqInfo := extractRequestInfo(c)

		t := time.Now()
		roundedDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) // round the time to date (day-month-year)

		matricsBson := &body{
			User:      user.ExtractRequestUser(c),
			Path:      c.Request.URL.Path,
			Method:    c.Request.Method,
			TimeStamp: t,
			Date:      roundedDate,
			TraceID:   currentTransaction.TraceContext().Trace.String(),
			AuthType:  reqInfo.authType,
			AppID:     reqInfo.appID,
		}
		_, _ = client.Index().
			Index(index).
			BodyJson(matricsBson).
			Do(c.Request.Context())
	}
}

func initESConfig() ([]es.ClientOptionFunc, string) {
	elasticURL := viper.GetString(configElasticsearchURL)
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: viper.GetBool(configTLSSkipVerify), // ignore expired SSL certificates
		},
	}
	httpClient := &http.Client{Transport: transCfg}

	elasticOpts := []es.ClientOptionFunc{
		es.SetURL(strings.Split(elasticURL, ",")...),
		es.SetSniff(viper.GetBool(configElasticsearchSniff)),
		es.SetHttpClient(httpClient),
	}

	elasticUser := viper.GetString(configElasticsearchUser)
	elasticPassword := viper.GetString(configElasticsearchPassword)
	if elasticUser != "" && elasticPassword != "" {
		elasticOpts = append(elasticOpts, es.SetBasicAuth(elasticUser, elasticPassword))
	}
	fmt.Printf("*************** metrics index: %v \n", viper.GetString(configElasticsearchIndex))

	return elasticOpts, viper.GetString(configElasticsearchIndex)
}

func extractRequestInfo(c *gin.Context) *extractedTokenInfo {
	var claims jwt.MapClaims
	authType := c.GetHeader(oauth.AuthTypeHeader)

	if authType == "" {
		authType = NoAuthType
	} else {
		spikeToken, err := oauth.ExtractTokenFromHeader(c)
		if err != nil {
			fmt.Printf("metrics token extraction error: %v", err)
		}

		spikeTokenObject, err := jwt.Parse(spikeToken, nil)
		// Check the error is not because of nil keyfunc
		if !((err != nil) && (err.(*jwt.ValidationError).Errors == jwt.ValidationErrorUnverifiable)) {
			fmt.Printf("metrics token parsing error: %v", err)
		}
		claims, _ = spikeTokenObject.Claims.(jwt.MapClaims) //the token claims should conform to MapClaims
	}

	appID := oauth.DriveAppID

	switch authType {
	case oauth.DropboxAuthTypeValue:
		appID = oauth.DropboxAppID
	case oauth.CargoAuthTypeValue:
		appID = oauth.CargoAuthTypeValue
	case oauth.ServiceAuthCodeTypeValue:
		claimAppID, ok := claims[ClientNameLabel].(string) // get the appID from the claims
		if !ok {
			fmt.Printf("metrics token parsing error: ClientNameLabel returned not ok")
			appID = UnknownAppID
		} else {
			appID = claimAppID
		}
	}
	finalInfo := &extractedTokenInfo{appID: appID, authType: authType}

	return finalInfo
}
