package file_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/meateam/api-gateway/file"
	"github.com/meateam/api-gateway/server"
	"github.com/meateam/api-gateway/upload"
	"github.com/sirupsen/logrus"
)

// Global variables
var (
	r                 *gin.Engine
	jwtKey            = []byte("pandora@drive")
	authToken         string
	rootFolderId      = ""
	rootChildFolderId string
	folderName        = "TEST"
	rootId            = ""
)

func generateJwtToken(key interface{}) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.MapClaims{
		"id":        "5cb72ad5b06cc14394c1d632",
		"firstName": "Elad",
		"lastName":  "Biran",
		"mail":      "elad@rabiran",
		"iat":       time.Now().Unix(),
	})

	return token.SignedString(jwtKey)
}

func init() {
	r, _ = server.NewRouter(logrus.New())

	var err error
	authToken, err = generateJwtToken(jwtKey)
	if err != nil {
		fmt.Printf("Error signing jwt token: %s \n", err)
	}

	rootFolderId, err = uploadFolder(folderName, rootId)
	if err != nil {
		fmt.Printf("Couldn't upload folder: %v\n", err)
	}

	rootChildFolderId, err = uploadFolder(folderName, rootFolderId)
	if err != nil {
		fmt.Printf("Couldn't upload folder: %v\n", err)
	}
}

func uploadFolder(folderName string, parentId string) (string, error) {
	params := url.Values{}
	params.Set(file.ParamFileParent, parentId)

	req, err := http.NewRequest(http.MethodPost, "/api/upload", bytes.NewBufferString(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("Couldn't create request: %v", err)
	}

	req.Header.Set(upload.ContentDispositionHeader, fmt.Sprintf("filename=%s", folderName))
	req.Header.Set(upload.ContentTypeHeader, upload.FolderContentType)
	req.AddCookie(&http.Cookie{Name: "kd-token", Value: authToken})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return "", fmt.Errorf("Expected to get status %d but instead got %d", http.StatusOK, w.Code)
	}
	return w.Body.String(), nil
}

func TestRouter_GetFilesByFolder(t *testing.T) {
	type args struct {
		params map[string]string
	}
	tests := []struct {
		name           string
		args           args
		wantStatusCode int
	}{
		{
			name: "Get files of root folder",
			args: args{
				params: map[string]string{
					file.ParamFileParent: rootId,
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "Get files of non root folder",
			args: args{
				params: map[string]string{
					file.ParamFileParent: rootChildFolderId,
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "Get files of non existing folder",
			args: args{
				params: map[string]string{
					file.ParamFileParent: "00000000bdff6cdf994390fd",
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "Get files of root folder filtered by type",
			args: args{
				params: map[string]string{
					file.ParamFileParent: rootFolderId,
					file.ParamFileType:   upload.FolderContentType,
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "Get files of root folder filtered by name",
			args: args{
				params: map[string]string{
					file.ParamFileParent: rootFolderId,
					file.ParamFileName:   folderName,
				},
			},
			wantStatusCode: http.StatusOK,
		},
		// TODO: filter by description
		// Below TODOs may be considered to be treated as range
		// TODO: filter by size
		// TODO: filter by date created
		// TODO: filter by date modified
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := url.Values{}
			for param, value := range tt.args.params {
				query.Set(param, value)
			}

			req, err := http.NewRequest(http.MethodGet, "/api/files", bytes.NewBufferString(query.Encode()))
			if err != nil {
				t.Fatalf("Couldn't create request: %v\n", err)
			}
			req.AddCookie(&http.Cookie{Name: "kd-token", Value: authToken})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("Expected to get status %d but instead got %d\n", http.StatusOK, w.Code)
			}
			fmt.Printf("%v\n", w.Body)
		})
	}
}
