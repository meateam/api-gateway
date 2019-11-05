package util

import (
	"context"

	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	qpb "github.com/meateam/file-service/proto/quota"
	ppb "github.com/meateam/permission-service/proto"
	upb "github.com/meateam/upload-service/proto"
	uspb "github.com/meateam/user-service/proto"
	pool "github.com/processout/grpc-go-pool"
)

// GetDownloadClient creates a download service grpc client, it returns a download service client
// and the connection used to create it, or an error if occurred.
func GetDownloadClient(ctx context.Context, p *pool.Pool) (dpb.DownloadClient, *pool.ClientConn, error) {
	clientConn, err := p.Get(ctx)
	if err != nil {
		return nil, nil, err
	}

	return dpb.NewDownloadClient(clientConn.ClientConn), clientConn, nil
}

// GetUploadClient creates a upload service grpc client, it returns a upload service client
// and the connection used to create it, or an error if occurred.
func GetUploadClient(ctx context.Context, p *pool.Pool) (upb.UploadClient, *pool.ClientConn, error) {
	clientConn, err := p.Get(ctx)
	if err != nil {
		return nil, nil, err
	}

	return upb.NewUploadClient(clientConn.ClientConn), clientConn, nil
}

// GetFileClient creates a file service grpc client, it returns a file service client
// and the connection used to create it, or an error if occurred.
func GetFileClient(ctx context.Context, p *pool.Pool) (fpb.FileServiceClient, *pool.ClientConn, error) {
	clientConn, err := p.Get(ctx)
	if err != nil {
		return nil, nil, err
	}

	return fpb.NewFileServiceClient(clientConn.ClientConn), clientConn, nil
}

// GetPermissionClient creates a permission service grpc client, it returns a permission service client
// and the connection used to create it, or an error if occurred.
func GetPermissionClient(ctx context.Context, p *pool.Pool) (ppb.PermissionClient, *pool.ClientConn, error) {
	clientConn, err := p.Get(ctx)
	if err != nil {
		return nil, nil, err
	}

	return ppb.NewPermissionClient(clientConn.ClientConn), clientConn, nil
}

// GetUserClient creates a user service grpc client, it returns a user service client
// and the connection used to create it, or an error if occurred.
func GetUserClient(ctx context.Context, p *pool.Pool) (uspb.UsersClient, *pool.ClientConn, error) {
	clientConn, err := p.Get(ctx)
	if err != nil {
		return nil, nil, err
	}

	return uspb.NewUsersClient(clientConn.ClientConn), clientConn, nil
}

// GetQuotaClient creates a quota service grpc client, it returns a quota service client
// and the connection used to create it, or an error if occurred.
func GetQuotaClient(ctx context.Context, p *pool.Pool) (qpb.QuotaServiceClient, *pool.ClientConn, error) {
	clientConn, err := p.Get(ctx)
	if err != nil {
		return nil, nil, err
	}

	return qpb.NewQuotaServiceClient(clientConn.ClientConn), clientConn, nil
}
