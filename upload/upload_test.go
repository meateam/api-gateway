package upload_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/internal/test"
	"github.com/meateam/api-gateway/server"
	"github.com/meateam/api-gateway/upload"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// Global variables
var (
	r         *gin.Engine
	authToken string
)

func init() {
	r, _ = server.NewRouter(logrus.New())

	var err error
	authToken, err = test.GenerateJwtToken()
	if err != nil {
		fmt.Printf("Error signing jwt token: %s \n", err)
	}
}

func TestRouter_UploadFolder(t *testing.T) {
	type args struct {
		filename              string
		setContentDisposition bool
	}
	tests := []struct {
		name           string
		args           args
		wantStatusCode int
	}{
		{
			name: "create folder",
			args: args{
				filename:              "TestFolderName",
				setContentDisposition: true,
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "create folder with blank name",
			args: args{
				filename:              "",
				setContentDisposition: true,
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "create folder without content-disposition header",
			args: args{
				filename:              "",
				setContentDisposition: false,
			},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/api/upload", nil)
			if err != nil {
				t.Fatalf("Couldn't create request: %v\n", err)
			}

			if tt.args.setContentDisposition {
				req.Header.Set(upload.ContentDispositionHeader, fmt.Sprintf("filename=%s", tt.args.filename))
			}
			req.Header.Set(upload.ContentTypeHeader, upload.FolderContentType)
			req.AddCookie(&http.Cookie{Name: "kd-token", Value: authToken})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("Expected to get status %d but instead got %d\n", tt.wantStatusCode, w.Code)
			}
		})
	}
}

func TestRouter_UploadMedia(t *testing.T) {
	type args struct {
		filename                 string
		parentFolder             string
		removeParentQueryKey     bool
		removeContentType        bool
		removeContentDisposition bool
		body                     []byte
	}

	generateFileName := func() string { return uuid.NewV4().String() }
	csvContentType := "text/csv"
	csvBody := []byte("h1,h2,h3\n,a1,a2,a3\nb1,b2,b3")

	tests := []struct {
		name           string
		args           args
		wantStatusCode int
	}{
		{
			name: "create file",
			args: args{
				filename: generateFileName(),
				body:     csvBody,
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "create empty file",
			args: args{
				filename: generateFileName(),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "create file without parent folder",
			args: args{
				filename:             generateFileName(),
				body:                 csvBody,
				removeParentQueryKey: true,
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "create file with empty filename",
			args: args{
				body: csvBody,
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "create file without content disposition (filename)",
			args: args{
				removeContentDisposition: true,
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "create file without content type",
			args: args{
				filename:          generateFileName(),
				removeContentType: true,
			},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(tt.args.body))
			if err != nil {
				t.Fatalf("Couldn't create request: %v\n", err)
			}

			if !tt.args.removeContentDisposition {
				req.Header.Set(upload.ContentDispositionHeader, fmt.Sprintf("filename=%s", tt.args.filename))
			}

			if !tt.args.removeContentType {
				req.Header.Set(upload.ContentTypeHeader, csvContentType)
			}

			if !tt.args.removeParentQueryKey {
				query := req.URL.Query()
				query.Set(upload.UploadTypeQueryKey, upload.MediaUploadType)
				query.Set(upload.ParentQueryKey, tt.args.parentFolder)
				req.URL.RawQuery = query.Encode()
			}

			req.AddCookie(&http.Cookie{Name: "kd-token", Value: authToken})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("Expected to get status %d but instead got %d, body: %s\n", tt.wantStatusCode, w.Code, w.Body)
			}
		})
	}
}
