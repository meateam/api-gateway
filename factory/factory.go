package factory

import (
	dlgpb "github.com/meateam/delegation-service/proto/delegation-service"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	qpb "github.com/meateam/file-service/proto/quota"
	ppb "github.com/meateam/permission-service/proto"
	ptpb "github.com/meateam/permit-service/proto"
	qapb "github.com/meateam/quota-approval-service/proto/quotaApproval"
	spb "github.com/meateam/search-service/proto"
	spkpb "github.com/meateam/spike-service/proto/spike-service"
	upb "github.com/meateam/upload-service/proto"
	usrpb "github.com/meateam/user-service/proto/users"
)

// SpikeClientFactory is a factory for the Spike GRPC client
type SpikeClientFactory = func() spkpb.SpikeClient

// DelegationClientFactory is a factory for the Delegation GRPC client
type DelegationClientFactory = func() dlgpb.DelegationClient

// DownloadClientFactory is a factory for the Download GRPC client
type DownloadClientFactory = func() dpb.DownloadClient

// FileClientFactory is a factory for the File GRPC client
type FileClientFactory = func() fpb.FileServiceClient

// UploadClientFactory is a factory for the Upload GRPC client
type UploadClientFactory = func() upb.UploadClient

// PermissionClientFactory is a factory for the Permission GRPC client
type PermissionClientFactory = func() ppb.PermissionClient

// PermitClientFactory is a factory for the Permit GRPC client
type PermitClientFactory = func() ptpb.PermitClient

// SearchClientFactory is a factory for the Search GRPC client
type SearchClientFactory = func() spb.SearchClient

// UserClientFactory is a factory for the User GRPC client
type UserClientFactory = func() usrpb.UsersClient

// QuotaClientFactory is a factory for the Quota GRPC client
type QuotaClientFactory = func() qpb.QuotaServiceClient

// QuotaApprovalClientFactory is a factory for the Quota GRPC client
type QuotaApprovalClientFactory = func() qapb.QuotaApprovalClient
