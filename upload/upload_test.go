package upload_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/internal/test"
	"github.com/meateam/api-gateway/server"
	"github.com/meateam/api-gateway/upload"
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
				t.Fatalf("Expected to get status %d but instead got %d\n", http.StatusOK, w.Code)
			}
		})
	}
}
