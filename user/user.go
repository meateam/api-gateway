package user

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	uspb "github.com/meateam/user-service/proto/users"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
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
)

//Router is a structure that handles users requests.
type Router struct {
	userClient uspb.UsersClient
	logger     *logrus.Logger
}

// User is a structure of an authenticated user.
type User struct {
	ID        string `json:"id"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Source    string `json:"source"`
	Bucket    string `json:"bucket"`
}

// NewRouter creates a new Router, and initializes clients of User Service
//  with the given connections. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	userConn *grpc.ClientConn,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.userClient = uspb.NewUsersClient(userConn)

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	rg.GET(fmt.Sprintf("/users/:%s", ParamUserID), r.GetUserByID)
	rg.GET("/users", r.SearchByName)
	rg.GET(fmt.Sprintf("/users/:%s/approverInfo", ParamUserID), r.GetApproverInfo)
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

	getUserByIDRequest := &uspb.GetByIDRequest{
		Id: userID,
	}

	user, err := r.userClient.GetUserByID(c.Request.Context(), getUserByIDRequest)

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

	findUserByNameRequest := &uspb.FindUserByNameRequest{
		Name: partialName,
	}

	user, err := r.userClient.FindUserByName(c.Request.Context(), findUserByNameRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetApproverInfo is the request handler for GET /users/:id/approverInfo
func (r *Router) GetApproverInfo(c *gin.Context) {
	reqUser := ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	userID := c.Param(ParamUserID)
	if userID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s field is required", ParamUserID))
		return
	}

	getApproverInfoRequest := &uspb.GetApproverInfoRequest{
		Id: userID,
	}

	info, err := r.userClient.GetApproverInfo(c.Request.Context(), getApproverInfoRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, info)
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
