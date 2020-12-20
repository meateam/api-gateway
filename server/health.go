package server

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meateam/gotenberg-go-client/v6"
	grpcPoolTypes "github.com/meateam/grpc-go-conn-pool/grpc/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// NewHealthChecker creates a new Health with default to false
func NewHealthChecker() *Health {
	return new(Health)
}

// Health is an atomic Boolean
// Its methods are all atomic, thus safe to be called by
// multiple goroutines simultaneously
// Note: When embedding into a struct, one should always use
// *Health to avoid copy
type Health int32

// Check checks healthiness of conns once in interval seconds.
func (h *Health) Check(
	interval int,
	rpcTimeout int,
	logger *logrus.Logger,
	gotenberg *gotenberg.Client,
	badConns chan<- *grpcPoolTypes.ConnPool,
	nonFatalConns []*grpcPoolTypes.ConnPool,
	fatalConns ...*grpcPoolTypes.ConnPool) {
	rpcTimeoutDuration := time.Duration(rpcTimeout) * time.Second
	for {
		// Check if the fatal connections are healthy.
		// If one is not healthy, it will fail the entire system.
		isHealthy := h.checkConnections(logger, fatalConns, badConns, true, rpcTimeout, rpcTimeoutDuration)

		// Check the non-fatal connections' health
		h.checkConnections(logger, nonFatalConns, badConns, false, rpcTimeout, rpcTimeoutDuration)

		if !gotenberg.Healthy() {
			logger.Printf("error: gotenberg at %s unhealthy", gotenberg.Hostname)
		}

		if isHealthy {
			h.SetHealthy()
		}

		time.Sleep(time.Second * time.Duration(interval))
	}
}

// checkConnections goes over an array of connections.
// If the array contains fatal connections and one of them failed,
// it will fail the api-gateway's healthcheck.
// Returns true iff all of the connections are healthy.
func (h *Health) checkConnections(
	logger *logrus.Logger,
	conns []*grpcPoolTypes.ConnPool,
	badConns chan<- *grpcPoolTypes.ConnPool,
	isFatal bool,
	rpcTimeout int,
	rpcTimeoutDuration time.Duration) bool {

	fatalString := "non-fatal"
	if isFatal {
		fatalString = "fatal"
	}

	isAllHealthy := true
	for _, pool := range conns {
		conn := (*pool).Conn()
		rpcCtx, rpcCancel := context.WithTimeout(context.Background(), rpcTimeoutDuration)
		defer rpcCancel()
		resp, err := healthpb.NewHealthClient(conn).Check(
			rpcCtx, &healthpb.HealthCheckRequest{Service: ""})
		targetMsg := fmt.Sprintf("target server %s", conn.Target())
		if err != nil {
			if stat, ok := status.FromError(err); ok && stat.Code() == codes.Unimplemented {
				logger.Printf(
					"error: %s does not implement the grpc health protocol (grpc.health.v1.Health) : %s",
					targetMsg, fatalString)
			} else if stat, ok := status.FromError(err); ok && stat.Code() == codes.DeadlineExceeded {
				logger.Printf("timeout: %s health rpc did not complete within %v : %s", targetMsg, rpcTimeout, fatalString)
			} else {
				logger.Printf("error: %s health rpc failed: %+v : %s", err, targetMsg, fatalString)
			}
			if isFatal {
				h.SetUnhealthy()
				isAllHealthy = false
			}
			badConns <- pool
		}

		if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			logger.Printf("%s service unhealthy (responded with %q) : %s",
				targetMsg, resp.GetStatus().String(), fatalString)
			if isFatal {
				h.SetUnhealthy()
				isAllHealthy = false
			}
		}
	}

	return isAllHealthy
}

// SetHealthy sets the Boolean to true
func (h *Health) SetHealthy() {
	atomic.StoreInt32((*int32)(h), 1)
}

// SetUnhealthy sets the Boolean to false
func (h *Health) SetUnhealthy() {
	atomic.StoreInt32((*int32)(h), 0)
}

// Get returns whether the Boolean is true
func (h *Health) Get() bool {
	return atomic.LoadInt32((*int32)(h)) == 1
}

// SetTo sets the boolean with given Boolean
func (h *Health) SetTo(yes bool) {
	if yes {
		atomic.StoreInt32((*int32)(h), 1)
	} else {
		atomic.StoreInt32((*int32)(h), 0)
	}
}

// SetToIf sets the Boolean to new only if the Boolean matches the old
// Returns whether the set was done
func (h *Health) SetToIf(old, new bool) (set bool) {
	var o, n int32
	if old {
		o = 1
	}
	if new {
		n = 1
	}
	return atomic.CompareAndSwapInt32((*int32)(h), o, n)
}

func (h *Health) healthCheck(c *gin.Context) {
	status := http.StatusServiceUnavailable
	if h.Get() {
		status = http.StatusOK
	}

	c.Status(status)
}
