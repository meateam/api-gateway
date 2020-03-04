package server

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meateam/gotenberg-go-client/v6"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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
	conns ...*grpc.ClientConn) {
	rpcTimeoutDuration := time.Duration(rpcTimeout) * time.Second
	for {
		flag := true
		for _, conn := range conns {
			func() {
				rpcCtx, rpcCancel := context.WithTimeout(context.Background(), rpcTimeoutDuration)
				defer rpcCancel()
				resp, err := healthpb.NewHealthClient(conn).Check(
					rpcCtx, &healthpb.HealthCheckRequest{Service: ""})
				targetMsg := fmt.Sprintf("target server %s", conn.Target())
				if err != nil {
					if stat, ok := status.FromError(err); ok && stat.Code() == codes.Unimplemented {
						logger.Printf(
							"error: %s does not implement the grpc health protocol (grpc.health.v1.Health)",
							targetMsg)
					} else if stat, ok := status.FromError(err); ok && stat.Code() == codes.DeadlineExceeded {
						logger.Printf("timeout: %s health rpc did not complete within %v", targetMsg, rpcTimeout)
					} else {
						logger.Printf("error: %s health rpc failed: %+v", err, targetMsg)
					}
					h.UnSet()
					flag = false
				}

				if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
					logger.Printf("%s service unhealthy (responded with %q)",
						targetMsg, resp.GetStatus().String())
					h.UnSet()
					flag = false
				}
			}()
		}

		if !gotenberg.Healthy() {
			logger.Printf("error: gotenberg at %s unhealthy", gotenberg.Hostname)
			h.UnSet()
			flag = false
		}

		if flag {
			h.Set()
		}

		time.Sleep(time.Second * time.Duration(interval))
	}
}

// Set sets the Boolean to true
func (h *Health) Set() {
	atomic.StoreInt32((*int32)(h), 1)
}

// UnSet sets the Boolean to false
func (h *Health) UnSet() {
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
