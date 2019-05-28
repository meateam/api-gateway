package logger

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.elastic.co/apm/module/apmhttp"
)

// Config is the configuration struct for the logger,
// Logger - a logrus Logger to use in the logger
// SkipPath - path to skip logging.
// SkipPathRegexp - a regex to skip paths.
type Config struct {
	Logger *logrus.Logger

	SkipPath       []string
	SkipPathRegexp *regexp.Regexp
}

// SetLogger initializes the logging middleware.
func SetLogger(config *Config) gin.HandlerFunc {
	if config != nil {
		config = &Config{}
	}
	var skip map[string]struct{}
	if length := len(config.SkipPath); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, path := range config.SkipPath {
			skip[path] = struct{}{}
		}
	}

	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		c.Next()
		// if skip contains the current path or the path matches the regex, skip it.
		if _, ok := skip[path]; ok ||
			(config.SkipPathRegexp != nil &&
				config.SkipPathRegexp.MatchString(path)) {
			return
		}

		end := time.Now()
		duration := end.Sub(start)
		end = end.UTC()

		msg := "Request"
		if len(c.Errors) > 0 {
			msg = c.Errors.String()
		}

		traceID := extractTraceParent(c)

		dumplogger := config.Logger.WithFields(
			logrus.Fields{
				"request.method":     c.Request.Method,
				"request.path":       path,
				"request.ip":         c.ClientIP(),
				"request.user-agent": c.Request.UserAgent(),
				"request.headers":    c.Request.Header,
				"trace.id":           traceID,
				"response.headers":   c.Writer.Header(),
				"response.status":    c.Writer.Status(),
				"duration":           duration,
			},
		)

		switch {
		case isWarning(c):
			{
				dumplogger.Warn(msg)
			}
		case isError(c):
			{
				dumplogger.Error(msg)
			}
		default:
			dumplogger.Info(msg)
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

func isWarning(c *gin.Context) bool {
	return c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError
}

func isError(c *gin.Context) bool {
	return c.Writer.Status() >= http.StatusInternalServerError
}
