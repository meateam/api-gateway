package main

import (
	"github.com/gin-gonic/gin"
	pb "github.com/meateam/file-service/protos"
	"google.golang.org/grpc"
)

type fileRouter struct {
	client         pb.FileServiceClient
	fileServiceURL string
}

func (fr *fileRouter) setup(r *gin.Engine) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(fr.fileServiceURL, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	fr.client = pb.NewFileServiceClient(conn)

	return conn, nil
}
