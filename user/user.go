package user

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
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

	// ParamApproverID is the name of the approver id param in the URL.
	ParamApproverID = "approverID"

	// ParamPartialName is the name of the partial user name param in URL.
	ParamPartialName = "partial"

	// ParamSearchByFlag is the name of the flag that determines which search to execute.
	ParamSearchByFlag = "searchBy"

	// ExternalUserSource is the value of the source field of user that indicated that the user is external
	ExternalUserSource = "external"

	// InternalUserSource is the value of the source field of user that indicated that the user is internal
	InternalUserSource = "internal"

	// ConfigBucketPostfix is the name of the environment variable containing the postfix for the bucket.
	ConfigBucketPostfix = "bucket_postfix"

	// TransactionUserLabel is the label of the custom transaction field : user.
	TransactionUserLabel = "user"
)

const (
	// SearchByNameFlag is a flag for searching by name
	SearchByNameFlag = iota

	// FindByMailFlag is a flag for finding by mail
	FindByMailFlag

	// FindByTFlag is a flag for finding by user T
	FindByTFlag
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
	rg.GET("/users", r.SearchByRouter)
	rg.GET(fmt.Sprintf("/users/:%s/canApproveToUser/:approverID", ParamUserID), r.CanApproveToUser)
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

	user, err := r.userClient().GetUserByID(c.Request.Context(), getUserByIDRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// SearchByRouter is the search by request router for GET /users
func (r *Router) SearchByRouter(c *gin.Context) {
	searchByFlag := stringToInt64(c.Query(ParamSearchByFlag))

	switch searchByFlag {
	case SearchByNameFlag:
		r.SearchByName(c)
	case FindByMailFlag:
		r.FindByMail(c)
	case FindByTFlag:
		r.FindByUserT(c)
	default:
		r.SearchByName(c)
	}
}

// FindByMail is the request handler for GET /users with flag FindByMailFlag
func (r *Router) FindByMail(c *gin.Context) {
	mail := c.Query(ParamPartialName)
	if mail == "" {
		c.String(http.StatusBadRequest, "mail required")
		return
	}

	findUserByMailRequest := &uspb.GetByMailRequest{
		Mail: mail,
	}

	user, err := r.userClient().GetUserByMail(c.Request.Context(), findUserByMailRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// FindByUserT is the request handler for GET /users with flag FindByTFlag
func (r *Router) FindByUserT(c *gin.Context) {
	userT := c.Query(ParamPartialName)
	if userT == "" {
		c.String(http.StatusBadRequest, "userT required")
		return
	}

	findUserByMailRequest := &uspb.GetByMailRequest{
		Mail: userT,
	}

	user, err := r.userClient().GetUserByMail(c.Request.Context(), findUserByMailRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, user)
	return
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

	user, err := r.userClient().FindUserByName(c.Request.Context(), findUserByNameRequest)

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

	info, err := r.userClient().GetApproverInfo(c.Request.Context(), getApproverInfoRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, info)
}

// CanApproveToUser is the request handler for GET /users/:id/canApproveToUser/:approverID
func (r *Router) CanApproveToUser(c *gin.Context) {
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

	approverID := c.Param(ParamApproverID)
	if approverID == "" {
		c.String(http.StatusBadRequest, fmt.Sprintf("%s field is required", ParamApproverID))
		return
	}

	canApproveToUserRequest := &uspb.CanApproveToUserRequest{
		ApproverID: approverID,
		UserID:     userID,
	}

	canApproveToUserInfo, err := r.userClient().CanApproveToUser(c.Request.Context(), canApproveToUserRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, canApproveToUserInfo)
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

// Converts a string to int64, 0 is returned on failure
func stringToInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		n = 0
	}
	return n
}
