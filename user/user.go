package user

import (
	"context"
	"regexp"
	"strings"
)

const (
	// ContextUserKey is the context key used to get and set the user's data in the context.
	ContextUserKey = "User"
)

// User is a structure of an authenticated user.
type User struct {
	ID        string
	FirstName string
	LastName  string
	Bucket    string
}

// ExtractRequestUser gets a context.Context and extracts the user's details from c.
func ExtractRequestUser(ctx context.Context) *User {
	contextUser := ctx.Value(ContextUserKey)
	if contextUser == nil {
		return nil
	}

	var reqUser User
	switch v := contextUser.(type) {
	case User:
		reqUser = v
		reqUser.Bucket = normalizeCephBucketName(reqUser.ID)
	default:
		return nil
	}

	return &reqUser
}

// NormalizeCephBucketName gets a bucket name and normalizes it
// according to ceph s3's constraints.
func normalizeCephBucketName(bucketName string) string {
	lowerCaseBucketName := strings.ToLower(bucketName)

	// Make a Regex for catching only letters and numbers.
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	return reg.ReplaceAllString(lowerCaseBucketName, "-")
}
