package logger

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.elastic.co/apm/module/apmhttp"
)

// Config is the configuration struct for the logger,
// Logger - a logrus Logger to use in the logger.
// SkipPath - path to skip logging.
// SkipPathRegexp - a regex to skip paths.
type Config struct {
	Logger             *logrus.Logger
	SkipBodyPath       []string
	SkipBodyPathRegexp *regexp.Regexp
	SkipPath           []string
	SkipPathRegexp     *regexp.Regexp
}

// SetLogger initializes the logging middleware.
func SetLogger(config *Config) gin.HandlerFunc {
	if config == nil {
		config = &Config{}
	}

	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	return func(c *gin.Context) {
		start := time.Now()
		fullPath := getRequestFullPath(c)

		requestBodyField := extractRequestBody(c, config, fullPath)

		c.Next()

		// If skip contains the current path or the path matches the regex, skip it.
		skip := mapStringSlice(config.SkipPath)
		if _, ok := skip[fullPath]; ok ||
			(config.SkipPathRegexp != nil &&
				config.SkipPathRegexp.MatchString(fullPath)) {
			return
		}

		end := time.Now().UTC()
		duration := end.Sub(start)
		msg := "Request"
		if len(c.Errors) > 0 {
			msg = c.Errors.String()
		}

		traceID := extractTraceParent(c)

		logger := config.Logger.WithFields(
			logrus.Fields{
				"request.method":     c.Request.Method,
				"request.path":       fullPath,
				"request.ip":         c.ClientIP(),
				"request.user-agent": c.Request.UserAgent(),
				"request.headers":    c.Request.Header,
				"request.body":       requestBodyField,
				"trace.id":           traceID,
				"response.headers":   c.Writer.Header(),
				"response.status":    c.Writer.Status(),
				"duration":           duration,
			},
		)

		switch {
		case isWarning(c):
			logger.Warn(msg)
		case isError(c):
			logger.Error(msg)
		default:
			logger.Info(msg)
		}
	}
}

func extractTraceParent(c *gin.Context) string {
	if values := c.Request.Header[apmhttp.TraceparentHeader]; len(values) == 1 && values[0] != "" {
		if traceContext, err := apmhttp.ParseTraceparentHeader(values[0]); err == nil {
			return traceContext.Trace.String()
		}
	}
	return ""
}

func mapStringSlice(s []string) map[string]struct{} {
	var mappedSlice map[string]struct{}
	if length := len(s); length > 0 {
		mappedSlice = make(map[string]struct{}, length)
		for _, v := range s {
			mappedSlice[v] = struct{}{}
		}
	}

	return mappedSlice
}

func extractRequestBody(c *gin.Context, config *Config, fullPath string) string {
	skipBody := mapStringSlice(config.SkipBodyPath)
	requestBodyField := ""
	if _, ok := skipBody[fullPath]; !ok ||
		!(config.SkipPathRegexp != nil &&
			config.SkipPathRegexp.MatchString(fullPath)) {
		if c.Request.ContentLength > 0 &&
			c.Request.ContentLength <= 1<<20 {
			var buf bytes.Buffer
			requestBody := io.TeeReader(c.Request.Body, &buf)

			if requestBody != nil {
				bodyBytes, err := ioutil.ReadAll(requestBody)
				c.Request.Body = ioutil.NopCloser(&buf)

				if err == nil {
					requestBodyField = string(bodyBytes)
				}
			}
		}
	}

	return requestBodyField
}

func getRequestFullPath(c *gin.Context) string {
	path := c.Request.URL.Path
	raw := c.Request.URL.RawQuery
	if raw != "" {
		path = path + "?" + raw
	}

	return path
}

func isWarning(c *gin.Context) bool {
	return c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError
}

func isError(c *gin.Context) bool {
	return c.Writer.Status() >= http.StatusInternalServerError
}
