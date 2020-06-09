package permit

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/meateam/api-gateway/file"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/api-gateway/oauth"
	"github.com/meateam/api-gateway/permission"
	"github.com/meateam/api-gateway/user"
	dpb "github.com/meateam/delegation-service/proto/delegation-service"
	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	ptpb "github.com/meateam/permit-service/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ParamFileID is the name of the file id param in URL.
	ParamFileID = "id"

	// ParamReqID is the name of the request id param in URL.
	ParamReqID = "requestId"

	// QueryDeleteUserPermission is the id of the user to delete its permission to a file.
	QueryDeleteUserPermission = "userId"
)

type createPermitRequest struct {
	FileName       string   `json:"fileName"`
	Users          []User   `json:"users,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Info           string   `json:"info,omitempty"`
	Approvers      []string `json:"approvers,omitempty"`
}

// User blabla
type User struct {
	ID       string `json:"id,omitempty"`
	FullName string `json:"full_name,omitempty"`
}

type updatePermitStatusRequest struct {
	Status string `json:"status,omitempty"`
}

// Router is a structure that handles permission requests.
type Router struct {
	permitClient     ptpb.PermitClient
	fileClient       fpb.FileServiceClient
	delegateClient   dpb.DelegationClient
	permissionClient ppb.PermissionClient
	oAuthMiddleware  *oauth.Middleware
	logger           *logrus.Logger
}

// NewRouter creates a new Router, and initializes clients of the quota Service
// with the given connection. If logger is non-nil then it will
// be set as-is, otherwise logger would default to logrus.New().
func NewRouter(
	permitConn *grpc.ClientConn,
	permissionConn *grpc.ClientConn,
	fileConn *grpc.ClientConn,
	delegateConn *grpc.ClientConn,
	oAuthMiddleware *oauth.Middleware,
	logger *logrus.Logger,
) *Router {
	// If no logger is given, use a default logger.
	if logger == nil {
		logger = logrus.New()
	}

	r := &Router{logger: logger}

	r.permitClient = ptpb.NewPermitClient(permitConn)
	r.permissionClient = ppb.NewPermissionClient(permissionConn)
	r.fileClient = fpb.NewFileServiceClient(fileConn)
	r.delegateClient = dpb.NewDelegationClient(delegateConn)

	r.oAuthMiddleware = oAuthMiddleware

	return r
}

// Setup sets up r and initializes its routes under rg.
func (r *Router) Setup(rg *gin.RouterGroup) {
	checkStatusScope := r.oAuthMiddleware.ScopeMiddleware(oauth.UpdatePermitStatusScope)

	rg.GET(fmt.Sprintf("/files/:%s/permits", ParamFileID), r.GetFilePermits)
	rg.PUT(fmt.Sprintf("/files/:%s/permits", ParamFileID), r.CreateFilePermits)
	rg.PATCH(fmt.Sprintf("/permits/:%s", ParamReqID), checkStatusScope, r.UpdateStatus)
}

// GetFilePermits is a route function for retrieving permits of a file
// File id is extracted from url params
func (r *Router) GetFilePermits(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !r.HandleUserFilePermission(c, fileID, permission.GetFilePermissionsRole) {
		return
	}

	permitRequest := &ptpb.GetPermitByFileIDRequest{FileID: fileID}
	permitsResponse, err := r.permitClient.GetPermitByFileID(c.Request.Context(), permitRequest)
	if err != nil && status.Code(err) != codes.Unimplemented {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	permits := permitsResponse.GetUserStatus()

	c.JSON(http.StatusOK, permits)
}

// CreateFilePermits creates permits for a given file and users
// File id is extracted from url params, role is extracted from request body.
func (r *Router) CreateFilePermits(c *gin.Context) {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	permitRequest := &createPermitRequest{}
	if err := c.ShouldBindJSON(permitRequest); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	fileID := c.Param(ParamFileID)
	if fileID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !r.HandleUserFilePermission(c, fileID, permission.CreateFilePermissionRole) {
		return
	}

	var userIDs []*ptpb.User
	for i := 0; i < len(permitRequest.Users); i++ {
		user := &ptpb.User{
			Id:       permitRequest.Users[i].ID,
			FullName: permitRequest.Users[i].FullName,
		}
		userIDs = append(userIDs, user)
	}

	createdPermits, err := r.permitClient.CreatePermit(c.Request.Context(), &ptpb.CreatePermitRequest{
		FileID:         fileID,
		FileName:       permitRequest.FileName,
		SharerID:       reqUser.ID,
		Users:          userIDs,
		Classification: permitRequest.Classification,
		Info:           permitRequest.Info,
		Approvers:      permitRequest.Approvers,
	})
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, createdPermits)

}

// UpdateStatus updates the permits status with the given request id
func (r *Router) UpdateStatus(c *gin.Context) {
	body := &updatePermitStatusRequest{}
	if err := c.ShouldBindJSON(body); err != nil {
		loggermiddleware.LogError(
			r.logger,
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unexpected body format")),
		)

		return
	}

	reqID := c.Param(ParamReqID)
	if reqID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	_, err := r.permitClient.UpdatePermitStatus(c.Request.Context(), &ptpb.UpdatePermitStatusRequest{
		ReqID:  reqID,
		Status: body.Status,
	})

	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))
		return
	}

	c.JSON(http.StatusOK, nil)

}

// HandleUserFilePermission gets a gin context and the id of the requested file,
// returns true if the user is permitted to operate on the file.
// Returns false if the user isn't permitted to operate on it,
// Returns false if any error occurred and logs the error.
func (r *Router) HandleUserFilePermission(c *gin.Context, fileID string, role ppb.Role) bool {
	reqUser := user.ExtractRequestUser(c)
	if reqUser == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return false
	}

	userFilePermission, _, err := file.CheckUserFilePermission(c.Request.Context(),
		r.fileClient,
		r.permissionClient,
		reqUser.ID,
		fileID,
		role)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return false
	}

	if userFilePermission == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return userFilePermission != ""
}
