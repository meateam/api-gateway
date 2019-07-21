/*
Package logger is used to create a logger middleware for gin using SetLogger and a given
configuration.
Configuration is done using environment variables:
ELASTICSEARCH_URL: The url of the elasticsearch server used by the server's logger.
	default: "http://localhost:9200", can be a list of urls.
ELASTICSEARCH_USER: The user to authenticate with to elasticsearch using basic auth.
	default: ""
ELASTICSEARCH_PASSWORD: The passowrd of the authenticated user in elasticsearch using basic auth.
Must be set with ELASTICSEARCH_USER.
	default: ""
LOG_INDEX: The index name of the server's logs in elasticsearch.
	default: "log"
LOG_LEVEL: The log level of the server's logger.
	default: 2 - ErrorLevel, see: logrus.AllLevels
TLS_SKIP_VERIFY: Ignore checking elasticsearch's certificate.
	default: "true"
HOST_NAME: The hostname to use with the elasticsearch hook of the logger.
	default: Executable name
*/
package logger
