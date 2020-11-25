package server

import (
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/user"
	es "github.com/olivere/elastic/v7"
	"github.com/spf13/viper"
	"go.elastic.co/apm"
)

type body struct {
	User      *user.User `json:"user,omitempty"`
	Path      string     `json:"path,omitempty"`
	Method    string     `json:"method,omitempty"`
	TimeStamp time.Time  `json:"timestamp,omitempty"`
	Date      time.Time  `json:"date,omitempty"`
	TraceID   string     `json:"traceID,omitempty"`
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

		t := time.Now()
		roundedDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) // round the time to date (day-month-year)

		_, _ = client.Index().
			Index(index).
			BodyJson(&body{
				User:      user.ExtractRequestUser(c),
				Path:      c.Request.URL.Path,
				Method:    c.Request.Method,
				TimeStamp: time.Now(),
				Date:      roundedDate,
				TraceID:   currentTransaction.TraceContext().Trace.String(),
			}).
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

	return elasticOpts, viper.GetString(configElasticsearchIndex)
}
