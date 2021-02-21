package factory

import (
	dpb "github.com/meateam/download-service/proto"
	drp "github.com/meateam/dropbox-service/proto/dropbox"
	fpb "github.com/meateam/file-service/proto/file"
	qpb "github.com/meateam/file-service/proto/quota"
	ppb "github.com/meateam/permission-service/proto"
	spb "github.com/meateam/search-service/proto"
	spkpb "github.com/meateam/spike-service/proto/spike-service"
	upb "github.com/meateam/upload-service/proto"
	usrpb "github.com/meateam/user-service/proto/users"
)

// SpikeClientFactory is a factory for the Spike GRPC client
type SpikeClientFactory = func() spkpb.SpikeClient

// DownloadClientFactory is a factory for the Download GRPC client
type DownloadClientFactory = func() dpb.DownloadClient

// FileClientFactory is a factory for the File GRPC client
type FileClientFactory = func() fpb.FileServiceClient

// UploadClientFactory is a factory for the Upload GRPC client
type UploadClientFactory = func() upb.UploadClient

// PermissionClientFactory is a factory for the Permission GRPC client
type PermissionClientFactory = func() ppb.PermissionClient

// SearchClientFactory is a factory for the Search GRPC client
type SearchClientFactory = func() spb.SearchClient

// UserClientFactory is a factory for the User GRPC client
type UserClientFactory = func() usrpb.UsersClient

// QuotaClientFactory is a factory for the Quota GRPC client
type QuotaClientFactory = func() qpb.QuotaServiceClient

// DropboxClientFactory is a factory for the Dropbox GRPC client
type DropboxClientFactory = func() drp.DropboxClient
