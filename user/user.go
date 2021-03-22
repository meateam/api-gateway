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

	// ParamApproverID is the name of the approver id param in the URL.
	ParamApproverID = "approverID"

	// ParamRequestContent is the name of the partial user name param in URL.
	ParamRequestContent = "content"

	// ParamSearchType is the name of the flag that determines which search to execute.
	ParamSearchType = "searchBy"

	// ExternalUserSource is the value of the source field of user that indicated that the user is external
	ExternalUserSource = "external"

	// InternalUserSource is the value of the source field of user that indicated that the user is internal
	InternalUserSource = "internal"

	// ConfigBucketPostfix is the name of the environment variable containing the postfix for the bucket.
	ConfigBucketPostfix = "bucket_postfix"

	// TransactionUserLabel is the label of the custom transaction field : user.
	TransactionUserLabel = "user"
)

type searchByEnum string

const (
	// SearchByName is an enum key for searching by name
	SearchByName searchByEnum = "SearchByName"

	// FindByMail is an enum key for finding by mail
	FindByMail searchByEnum = "FindByMail"

	// FindByT is an enum key for finding by user T
	FindByT searchByEnum = "FindByT"
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

// usersResponse is a structure of a the response returned from users search
type usersResponse struct {
	Users []*uspb.User `json:"users"`
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
	searchBy := searchByEnum((c.Query(ParamSearchType)))

	switch searchBy {
	case SearchByName:
		r.SearchByName(c)
	case FindByMail:
		r.FindByMail(c)
	case FindByT:
		r.FindByUserT(c)
	default:
		r.SearchByName(c)
	}
}

// FindByMail is the request handler for GET /users with flag FindByMailFlag
func (r *Router) FindByMail(c *gin.Context) {
	mail := c.Query(ParamRequestContent)
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

	usersResponse := encapsulateUserResponse(user)

	c.JSON(http.StatusOK, usersResponse)
}

// FindByUserT is the request handler for GET /users with flag FindByTFlag
func (r *Router) FindByUserT(c *gin.Context) {
	userT := c.Query(ParamRequestContent)
	if userT == "" {
		c.String(http.StatusBadRequest, "userT required")
		return
	}

	// User service accepts the same route for mails and userT - user search
	findUserByTRequest := &uspb.GetByMailRequest{
		Mail: userT,
	}

	user, err := r.userClient().GetUserByMail(c.Request.Context(), findUserByTRequest)

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	usersResponse := encapsulateUserResponse(user)

	c.JSON(http.StatusOK, usersResponse)
}

// SearchByName is the request handler for GET /users
func (r *Router) SearchByName(c *gin.Context) {
	partialName := c.Query(ParamRequestContent)
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

// encapulateUserResponse gets a user of type *uspb.GetUserResponse and encapsulate it to type usersResponse
func encapsulateUserResponse(user *uspb.GetUserResponse) usersResponse {
	var users []*uspb.User

	users = append(users, user.GetUser())

	usersResponse := usersResponse{Users: users}

	return usersResponse
}
