package file

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zip"
	"github.com/meateam/download-service/download"
	dpb "github.com/meateam/download-service/proto"
	fpb "github.com/meateam/file-service/proto/file"
	"github.com/spf13/viper"
)

const (
	// configMaxZippingSize is the label for maximum permitted size of all files before zipping.
	configMaxZippingSize = "max_zipping_size"

	// configMaxZippingAmount is the label for  maximum permitted amount of all files for zipping.
	configMaxZippingAmount = "max_zipping_amount"
)

// SubDir is the struct describing a directory and it's descendants.
// fileObject is the directory's file object
// mappedPaths maps the fileIDs of the permitted descendants to their paths
type SubDir struct {
	fileObject           *fpb.File
	mappedPaths          map[string]string
	permittedDescendants []*fpb.GetDescendantsByIDResponse_Descendant
}

// VerifiedChildren is the struct describing the nested children of an array of files
// SubDirs are the direct folders of the multiple files requested
// RegularFiles are the direct files of the multiple files requested which are not folders
type VerifiedChildren struct {
	subDirs          []*SubDir
	regularFiles     []*fpb.File
	totalFilesSize   int64
	totalFilesAmount int64
}

// zipMulipleFiles goes over an array of files and adds to a zip.Writer
func (r *Router) zipMultipleFiles(c *gin.Context, files []*fpb.File) error {
	archive := zip.NewWriter(c.Writer)
	defer archive.Close()

	// Verify all of the files and their subdirectories, and return them as a single object
	verifiedChildren, err := r.verifyZipSize(c, files)
	if err != nil {
		return err
	}

	// Go over the directories requested and zip them and their nested files to the writer
	for _, directory := range verifiedChildren.subDirs {
		err = r.zipFolderToWriter(c, directory, archive)
		if err != nil {
			return err
		}
	}

	// Go over the regular files requested and zip them to the writer
	for _, file := range verifiedChildren.regularFiles {
		err := r.addFileToArchive(c, archive, file.GetName(), file)
		if err != nil {
			return err
		}
	}

	return nil
}

// zipFolderToWriter receives the mappingPaths, the descendants array and the folder -
// and creates the zip from said folder.
// The zip is then inserted to the writer(archive) and sent to the client.
func (r *Router) zipFolderToWriter(c *gin.Context, directory *SubDir, archive *zip.Writer) error {

	folderHeader := &zip.FileHeader{
		Name:   directory.mappedPaths[directory.fileObject.GetId()],
		Method: zip.Deflate,
	}

	folderHeader.Modified = time.Unix(directory.fileObject.GetUpdatedAt()/time.Second.Milliseconds(), 0)
	if _, err := archive.CreateHeader(folderHeader); err != nil {
		return err
	}

	for _, descendant := range directory.permittedDescendants {
		err := r.addFileToArchive(
			c,
			archive,
			directory.mappedPaths[descendant.GetFile().GetId()],
			descendant.GetFile(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// addFileToArchive receives a file and adds it to a given archive with the given path
func (r Router) addFileToArchive(c *gin.Context, archive *zip.Writer, filePath string, file *fpb.File) error {
	header := &zip.FileHeader{
		Name:               filePath,
		Method:             zip.Deflate,
		UncompressedSize64: uint64(file.GetSize()),
		Modified:           time.Unix(file.GetUpdatedAt()/time.Second.Milliseconds(), 0),
	}

	if file.GetType() == FolderContentType {
		_, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

	} else {
		buffer := make([]byte, download.PartSize)

		stream, err := r.downloadClient().Download(c.Request.Context(), &dpb.DownloadRequest{
			Key:    file.GetKey(),
			Bucket: file.GetBucket(),
		})
		if err != nil {
			return err
		}

		readCloser := download.NewStreamReadCloser(stream)

		if hasStringSuffixInSlice(filePath, standardExcludeCompressExtensions) ||
			hasPattern(standardExcludeCompressContentTypes, file.GetType()) {
			// We strictly disable compression for standard extensions/content-types.
			header.Method = zip.Store
		}

		if err := zipFileToWriter(readCloser, archive, header, buffer); err != nil {
			return err
		}
	}

	return nil
}

// zipFileToWriter zips the read cluster rc into archive according to the given header.
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

// verifyZipSize verifies that the zip meets the standards, and returns the VerifiedChildren object
func (r Router) verifyZipSize(c *gin.Context, files []*fpb.File) (*VerifiedChildren, error) {
	MaxZippingAmount := viper.GetInt64(configMaxZippingAmount)
	MaxZippingSize := viper.GetInt64(configMaxZippingSize)

	totalFilesSize := int64(0)
	totalFilesAmount := int64(len(files))
	subDirs := make([]*SubDir, 0, len(files))
	regularFiles := make([]*fpb.File, 0, len(files))

	if totalFilesAmount > MaxZippingAmount {
		err := fmt.Errorf("requested too many files. Requested: %d , Max number: %d", totalFilesAmount, MaxZippingAmount)

		return nil, err
	}

	for _, file := range files {
		if file.GetType() == FolderContentType {
			mappedPaths, permittedDescendants, folderSize, err := r.getPermittedDescendantsMapPath(c, file)
			if err != nil {
				return nil, err
			}

			totalFilesSize += folderSize
			if totalFilesSize > MaxZippingSize {
				err := fmt.Errorf("requested files are too large. Max size: %s", ByteCountIEC(MaxZippingSize))
				c.String(http.StatusRequestEntityTooLarge, err.Error())
				return nil, err
			}

			totalFilesAmount += int64(len(permittedDescendants))
			if totalFilesAmount > MaxZippingAmount {
				err := fmt.Errorf("requested too many files. Max number: %d", MaxZippingAmount)
				c.String(http.StatusRequestEntityTooLarge, err.Error())
				return nil, err
			}

			subDirs = append(subDirs, &SubDir{
				fileObject:           file,
				mappedPaths:          mappedPaths,
				permittedDescendants: permittedDescendants,
			})

		} else {
			totalFilesSize += file.GetSize()
			if totalFilesSize > MaxZippingSize {
				err := fmt.Errorf("requested files are too large. Max size: %s", ByteCountIEC(MaxZippingSize))
				c.String(http.StatusRequestEntityTooLarge, err.Error())
				return nil, err
			}

			regularFiles = append(regularFiles, file)
		}

	}
	verifiedChildren := &VerifiedChildren{
		subDirs:          subDirs,
		regularFiles:     regularFiles,
		totalFilesSize:   int64(totalFilesSize),
		totalFilesAmount: int64(totalFilesAmount),
	}

	return verifiedChildren, nil
}

// getPermittedDescendantsMapPath receives a root folder and returns:
// 1. A map of the ids to the permitted descendants.
// 2. The permitted descendants array.
// 3. The total size of the files in the folder.
// 4. An error if there was one.
func (r *Router) getPermittedDescendantsMapPath(c *gin.Context, folder *fpb.File) (map[string]string, []*fpb.GetDescendantsByIDResponse_Descendant, int64, error) {
	res, err := r.fileClient().GetDescendantsByID(c.Request.Context(), &fpb.GetDescendantsByIDRequest{Id: folder.GetId()})
	if err != nil {
		return nil, nil, 0, err
	}

	totalSize := int64(0)
	descendants := res.GetDescendants()
	permittedDescendants := make([]*fpb.GetDescendantsByIDResponse_Descendant, 0, len(descendants))
	// Go over the files and filter those who are not permitted
	for i := 0; i < len(descendants); i++ {
		if role, _ := r.HandleUserFilePermission(c, descendants[i].GetFile().GetId(), DownloadRole); role == "" {
			continue
		}
		totalSize += int64(descendants[i].GetFile().GetSize())

		permittedDescendants = append(permittedDescendants, descendants[i])
	}

	// de-allocate memory
	descendants = nil

	return mapFolderDescendantPath(folder, permittedDescendants), permittedDescendants, totalSize, nil
}

// mapFolderDescendantPath gets a root folder and its descendants and returns a map of the root folder and
// its descendants path mapped by their IDs.
func mapFolderDescendantPath(folder *fpb.File, descendants []*fpb.GetDescendantsByIDResponse_Descendant) map[string]string {
	filePathMap := make(map[string]string, len(descendants)+1)
	filePathMap[folder.GetId()] = folder.GetName() + "/"
	currentBase := folder.GetId()
	currentBaseFileIndex := 0

	// While there's a file that its path hasn't been set continue to the next descendant and set its path.
	for len(filePathMap) < len(descendants)+1 {
		for i := 0; i < len(descendants); i++ {

			// Check if we arrived to the current file and if we didn't already add the file.
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

/************************************************************************************
********************************* UTILITY FUNCTIONS *********************************
*************************************************************************************/

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

// Recursively matches the pattern given to the string.
// Simple means '*'.
// returns 'true' for a match and 'false' otherwise
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

// ByteCountIEC converts a size in bytes to a human-readable string in binary format
func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
