package file

import (
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/klauspost/compress/zip"
	loggermiddleware "github.com/meateam/api-gateway/logger"
	"github.com/meateam/download-service/download"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	"google.golang.org/grpc/status"
)

// mapFolderDescendantPath gets a root folder and its descendants and returns a map of the root folder's and
// its descendants path mapped by their IDs.
func mapFolderDescendantPath(folder *fpb.File, descendants []*fpb.GetDescendantsByIDResponse_Descendant) map[string]string {
	filePathMap := make(map[string]string, len(descendants) + 1)
	filePathMap[folder.GetId()] = folder.GetName() + "/"
	currentBase := folder.GetId()
	currentBaseFileIndex := 0

	// While there's a file that its path hasn't been set continue to the next descendant and set its path.
	for len(filePathMap) < len(descendants) + 1 {
		for i := 0; i < len(descendants); i++ {
			if descendants[i].GetParent().GetId() == currentBase && filePathMap[currentBase] != "" {
				path := filePathMap[currentBase] + descendants[i].GetFile().GetName()
				if descendants[i].GetFile().GetType() == FolderContentType {
					path += "/"
				}
	
				filePathMap[descendants[i].GetFile().GetId()] = path
			}
		}
		
		currentBase = descendants[currentBaseFileIndex].GetFile().GetId()
		currentBaseFileIndex++
	}

	return filePathMap
}

func (r *Router) getPermittedDescendantsMapPath(
	c *gin.Context,
	folder *fpb.File) (map[string]string, []*fpb.GetDescendantsByIDResponse_Descendant, error) {
	res, err := r.fileClient.GetDescendantsByID(c.Request.Context(), &fpb.GetDescendantsByIDRequest{Id: folder.GetId()})
	if err != nil {
		return nil, nil, err
	}

	descendants := res.GetDescendants()
	permittedDescendants := make([]*fpb.GetDescendantsByIDResponse_Descendant, 0, len(descendants))
	for i := 0; i < len(descendants); i++ {
		if role, _ := r.HandleUserFilePermission(c, descendants[i].GetFile().GetId(), DownloadRole); role == "" {
			continue
		}

		permittedDescendants = append(permittedDescendants, descendants[i])
	}

	descendants = nil
	permittedDescendants = permittedDescendants[:len(permittedDescendants)]

	return mapFolderDescendantPath(folder, permittedDescendants), permittedDescendants, nil
}

func (r *Router) zipFolderToWriter(
	c *gin.Context,
	mappedPaths map[string]string,
	descendants []*fpb.GetDescendantsByIDResponse_Descendant,
	folder *fpb.File,
	archive *zip.Writer) error {
	folderHeader := &zip.FileHeader{
		Name: mappedPaths[folder.GetId()],
		Method: zip.Deflate,
	}

	folderHeader.SetModTime(time.Unix(folder.GetUpdatedAt()/time.Second.Milliseconds(), 0))
	if _, err := archive.CreateHeader(folderHeader); err != nil {
		return err
	}

	buffer := make([]byte, download.PartSize)
	for _, descendant := range descendants {
		if descendant.GetFile().GetType() == FolderContentType {
			header := &zip.FileHeader{
				Name:               mappedPaths[descendant.GetFile().GetId()],
				Method:             zip.Deflate,
				UncompressedSize64: uint64(descendant.GetFile().GetSize()),
			}
			header.SetModTime(time.Unix(descendant.GetFile().GetUpdatedAt()/time.Second.Milliseconds(), 0))

			_, err := archive.CreateHeader(header)
			if err != nil {
				return err
			}
		} else {
			stream, err := r.downloadClient.Download(c.Request.Context(), &dpb.DownloadRequest{
				Key:    descendant.GetFile().GetKey(),
				Bucket: descendant.GetFile().GetBucket(),
			})
	
			if err != nil {
				return err
			}
	
			readCloser := download.NewStreamReadCloser(stream)
			header := &zip.FileHeader{
				Name:               mappedPaths[descendant.GetFile().GetId()],
				Method:             zip.Deflate,
				UncompressedSize64: uint64(descendant.GetFile().GetSize()),
			}
			header.SetModTime(time.Unix(descendant.GetFile().GetUpdatedAt()/time.Second.Milliseconds(), 0))
	
			if hasStringSuffixInSlice(mappedPaths[descendant.GetFile().GetId()], standardExcludeCompressExtensions) ||
				hasPattern(standardExcludeCompressContentTypes, descendant.GetFile().GetType()) {
				// We strictly disable compression for standard extensions/content-types.
				header.Method = zip.Store
			}
	
			if err := zipFileToWriter(readCloser, archive, header, buffer); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Router) downloadFolder(c *gin.Context, folder *fpb.File) {
	mappedPaths, permittedDescendants, err := r.getPermittedDescendantsMapPath(c, folder)
	if err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}

	now := strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339), ":", "")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header(ContentDispositionHeader, "attachment; filename="+folder.GetName() + "-" + now)

	archive := zip.NewWriter(c.Writer)
	defer archive.Close()
	if err := r.zipFolderToWriter(c, mappedPaths, permittedDescendants, folder, archive); err != nil {
		httpStatusCode := gwruntime.HTTPStatusFromCode(status.Code(err))
		loggermiddleware.LogError(r.logger, c.AbortWithError(httpStatusCode, err))

		return
	}
}

func (r *Router) zipMulipleFiles(c *gin.Context, files []*fpb.File) error {
	archive := zip.NewWriter(c.Writer)
	defer archive.Close()

	buffer := make([]byte, download.PartSize)
	for _, file := range files {
		if file.GetType() == FolderContentType {
			mappedPaths, permittedDescendants, err := r.getPermittedDescendantsMapPath(c, file)
			if err != nil {
				return err
			}

			if err := r.zipFolderToWriter(c, mappedPaths, permittedDescendants, file, archive); err != nil {
				return err
			}
		} else {
			stream, err := r.downloadClient.Download(c.Request.Context(), &dpb.DownloadRequest{
				Key:    file.GetKey(),
				Bucket: file.GetBucket(),
			})
	
			if err != nil {
				return err
			}
	
			readCloser := download.NewStreamReadCloser(stream)
			header := &zip.FileHeader{
				Name:               file.GetName(),
				Method:             zip.Deflate,
				UncompressedSize64: uint64(file.GetSize()),
			}
			header.SetModTime(time.Unix(file.GetUpdatedAt()/time.Second.Milliseconds(), 0))
	
			if hasStringSuffixInSlice(file.GetName(), standardExcludeCompressExtensions) ||
				hasPattern(standardExcludeCompressContentTypes, file.GetType()) {
				// We strictly disable compression for standard extensions/content-types.
				header.Method = zip.Store
			}
	
			if err := zipFileToWriter(readCloser, archive, header, buffer); err != nil {
				return err
			}
		}
	}

	return nil
}

// zipFileToWriter zips r into archive according to the given header.
func zipFileToWriter(r io.ReadCloser, archive *zip.Writer, header *zip.FileHeader, buffer []byte) error {
	if buffer == nil {
		buffer = make([]byte, download.PartSize)
	}

	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}

	if _, err = io.CopyBuffer(writer, r, buffer); err != nil {
		return err
	}

	return nil
}

// Utility which returns if a string is present in the list.
// Comparison is case insensitive.
func hasStringSuffixInSlice(str string, list []string) bool {
	str = strings.ToLower(str)
	for _, v := range list {
		if strings.HasSuffix(str, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

// Returns true if any of the given wildcard patterns match the matchStr.
func hasPattern(patterns []string, matchStr string) bool {
	for _, pattern := range patterns {
		if ok := matchSimple(pattern, matchStr); ok {
			return true
		}
	}
	return false
}

// MatchSimple - finds whether the text matches/satisfies the pattern string.
// supports only '*' wildcard in the pattern.
// considers a file system path as a flat name space.
func matchSimple(pattern, name string) bool {
	if pattern == "" {
		return name == pattern
	}
	if pattern == "*" {
		return true
	}
	rname := make([]rune, 0, len(name))
	rpattern := make([]rune, 0, len(pattern))
	for _, r := range name {
		rname = append(rname, r)
	}
	for _, r := range pattern {
		rpattern = append(rpattern, r)
	}
	simple := true // Does only wildcard '*' match.
	return deepMatchRune(rname, rpattern, simple)
}

func deepMatchRune(str, pattern []rune, simple bool) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		default:
			if len(str) == 0 || str[0] != pattern[0] {
				return false
			}
		case '?':
			if len(str) == 0 && !simple {
				return false
			}
		case '*':
			return deepMatchRune(str, pattern[1:], simple) ||
				(len(str) > 0 && deepMatchRune(str[1:], pattern, simple))
		}
		str = str[1:]
		pattern = pattern[1:]
	}
	return len(str) == 0 && len(pattern) == 0
}