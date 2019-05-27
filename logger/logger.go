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
// UTC - boolean stating whether to use UTC time zone or local.
// SkipPath - path to skip logging.
// SkipPathRegexp - a regex to skip paths.
type Config struct {
	Logger *logrus.Logger
	// UTC a boolean stating whether to use UTC time zone or local.
	UTC            bool
	SkipPath       []string
	SkipPathRegexp *regexp.Regexp
}

// SetLogger initializes the logging middleware.
func SetLogger(config ...Config) gin.HandlerFunc {
	var newConfig Config
	if len(config) > 0 {
		newConfig = config[0]
	}
	var skip map[string]struct{}
	if length := len(newConfig.SkipPath); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, path := range newConfig.SkipPath {
			skip[path] = struct{}{}
		}
	}

	var sublog *logrus.Logger
	if newConfig.Logger == nil {
		sublog = logrus.New()
	} else {
		sublog = newConfig.Logger
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		c.Next()
		track := true

		if _, ok := skip[path]; ok {
			track = false
		}

		if track &&
			newConfig.SkipPathRegexp != nil &&
			newConfig.SkipPathRegexp.MatchString(path) {
			track = false
		}

		if track {
			end := time.Now()
			latency := end.Sub(start)
			if newConfig.UTC {
				end = end.UTC()
			}

			msg := "Request"
			if len(c.Errors) > 0 {
				msg = c.Errors.String()
			}

			traceID := ""
			if values := c.Request.Header[apmhttp.TraceparentHeader]; len(values) == 1 && values[0] != "" {
				if traceContext, err := apmhttp.ParseTraceparentHeader(values[0]); err == nil {
					traceID = traceContext.Trace.String()
				}
			}

			dumplogger := sublog.WithFields(
				logrus.Fields{
					"status":     c.Writer.Status(),
					"method":     c.Request.Method,
					"path":       path,
					"ip":         c.ClientIP(),
					"latency":    latency,
					"user-agent": c.Request.UserAgent(),
					"headers":    c.Request.Header,
					"trace.id":   traceID,
				},
			)

			switch {
			case c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError:
				{
					dumplogger.Warn(msg)
				}
			case c.Writer.Status() >= http.StatusInternalServerError:
				{
					dumplogger.Error(msg)
				}
			default:
				dumplogger.Info(msg)
			}
		}
	}
}
