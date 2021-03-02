package user

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/factory"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	uspb "github.com/meateam/user-service/proto/users"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.elastic.co/apm"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/status"
)

const (
	// ContextUserKey is the context key used to get and set the user's data in the context.
	ContextUserKey = "User"

	// ParamUserID is the name of the user id param in URL.
	ParamUserID = "id"

	// ParamPartialName is the name of the partial user name param in URL.
	ParamPartialName = "partial"

	// ExternalUserSource is the value of the source field of user that indicated that the user is external
	ExternalUserSource = "external"

	// InternalUserSource is the value of the source field of user that indicated that the user is internal
	InternalUserSource = "internal"

	// ConfigBucketPostfix is the name of the environment variable containing the postfix for the bucket.
	ConfigBucketPostfix = "bucket_postfix"

	// TransactionUserLabel is the label of the custom transaction field : user.
	TransactionUserLabel = "user"

	// HeaderDestionation is the header used to get and set the external destination.
	HeaderDestionation = "destination"

	// TODO: add to env ?

	// TomcalDest is the destination of the dropbox.
	TomcalDest = "TOMCAL"

	// CtsDest is the destination of the dropbox.
	CtsDest = "CTS"
)

//Router is a structure that handles users requests.
type Router struct {
	// UserClientFactory
	userClient factory.UserClientFactory

	logger *logrus.Logger
}

// User is a structure of an authenticated user.
type User struct {
	ID          string `json:"id"`
	FirstName   string `json:"firstname"`
	LastName    string `json:"lastname"`
	Source      string `json:"source"`
	Bucket      string `json:"bucket"`
	DisplayName string `json:"displayName"`
	CurrentUnit string `json:"currentUnit"`
	Rank        string `json:"rank"`
	Job         string `json:"job"`
}

// NewRouter creates a new Router, and initializes clients of User Service
//  with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	userConn *grpcPoolTypes.ConnPool,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.userClient = func() uspb.UsersClient {
		return uspb.NewUsersClient((*userConn).Conn())
	}

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET(fmt.Sprintf("/users/:%s", ParamUserID), r.GetUserByID)
	rg.GET("/users", r.SearchByName)
}

// GetUserByID is the request handler for GET /users/:id
func (r *Router) GetUserByID(c *gin.Context) {
	reqUser := ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	userID := c.Param(ParamUserID)
	if userID == "" {
		c.String(http.StatusBadRequest, "id is required")
		return
	}

	destination := c.GetHeader(HeaderDestionation)
	if destination == "" {
		destination = TomcalDest
	}
	if destination != "" && destination != CtsDest && destination != TomcalDest {
		c.String(http.StatusBadRequest, fmt.Sprintf("destination %s doesnt supported", destination))
		return
	}

	getUserByIDRequest := &uspb.GetByIDRequest{
		Id:          userID,
		Destination: destination,
	}

	user, err := r.userClient().GetUserByID(c.Request.Context(), getUserByIDRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// SearchByName is the request handler for GET /users
func (r *Router) SearchByName(c *gin.Context) {
	partialName := c.Query(ParamPartialName)
	if partialName == "" {
		c.String(http.StatusBadRequest, "partial name required")
		return
	}

	destination := c.GetHeader(HeaderDestionation)
	if destination == "" {
		destination = TomcalDest
	}
	if destination != "" && destination != CtsDest && destination != TomcalDest {
		c.String(http.StatusBadRequest, fmt.Sprintf("destination %s doesnt supported", destination))
		return
	}
	
	findUserByNameRequest := &uspb.FindUserByNameRequest{
		Name:        partialName,
		Destination: destination,
	}

	user, err := r.userClient().FindUserByName(c.Request.Context(), findUserByNameRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// ExtractRequestUser gets a context.Context and extracts the user's details from c.
func ExtractRequestUser(ctx context.Context) *User {
	contextUser := ctx.Value(ContextUserKey)
	if contextUser == nil {
		return nil
	}

	var reqUser User
	switch v := contextUser.(type) {
	case User:
		reqUser = v
		reqUser.Bucket = normalizeCephBucketName(reqUser.ID)
	default:
		return nil
	}

	return &reqUser
}

// normalizeCephBucketName gets a bucket name and normalizes it
// according to ceph s3's constraints.
func normalizeCephBucketName(bucketName string) string {
	postfix := viper.GetString(ConfigBucketPostfix)
	lowerCaseBucketName := strings.ToLower(bucketName + postfix)

	// Make a Regex for catching only letters and numbers.
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	return reg.ReplaceAllString(lowerCaseBucketName, "-")
}

// IsExternalUser gets a userID and returns true if user is from an external source.
// Otherwise, returns false.
// Currently just and all of the external users don't have a valid mongoID .
func IsExternalUser(userID string) bool {
	_, err := primitive.ObjectIDFromHex(userID)
	return err != nil
}

// SetApmUser adds a user to the current apm transaction.
func SetApmUser(ctx *gin.Context, user User) {
	currentTransaction := apm.TransactionFromContext(ctx.Request.Context())

	currentTransaction.Context.SetCustom(TransactionUserLabel, user)
	currentTransaction.Context.SetUserID(user.ID)

	if user.DisplayName != "" {
		currentTransaction.Context.SetUserEmail(user.DisplayName)
	}
}
