package commits

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
)

//uploader is an interface for uploading repository
type Uploader interface {
	UploadRepo(repoID uint, src string, r *http.Request) (string, error)
}

//S3Uploader defines the mechanism to upload data to S3
type S3Uploader struct {
	Client            *s3.S3
	S3ManagerUploader *s3manager.Uploader
	Bucket            string
}

// File Uploader isn't actually an uploader but implements the interface in
// order to allow the workflow to be done to completion on a local machine
// without S3
type FileUploader struct {
	BaseDir string
}

// This is Basically a dummy function that returns the src, but allows offline
// development without S3 and satisfies the interface
func (u *FileUploader) UploadRepo(repoID uint, src string, r *http.Request) (string, error) {
	return src, nil
}

//NewS3Uploader creates a method to obtain a new S3 uploader
func NewS3Uploader() *S3Uploader {
	cfg := config.Get()
	sess := session.Must(session.NewSession())
	client := s3.New(sess)
	uploader := s3manager.NewUploader(sess)
	return &S3Uploader{
		Client:            client,
		S3ManagerUploader: uploader,
		Bucket:            cfg.BucketName,
	}
}

// UploadReopo uploads the repo to a backing object storage bucket
// the repository is uploaded to
//  bucket/$account/$name/
func (u *S3Uploader) UploadRepo(repoID uint, src string, r *http.Request) (string, error) {

	account, err := common.GetAccount(r)
	if err != nil {
		return "", err
	}
	s3path := fmt.Sprintf("s3://%s/%s/%d", u.Bucket, account, repoID)

	// FIXME: might experiment with doing this concurrently but I've read that
	//		  that can get you rate limited by S3 pretty quickly so we'll mess
	//		  with that later.
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		err = u.UploadFileToS3(path, filepath.Join(account, "/", strconv.FormatUint(uint64(repoID), 10)))
		if err != nil {
			return err
		}
		return nil
	})

	return s3path, nil
}

// UploadFileToS3 takes a FILename path as a string and then uploads that to
// the supplied location in s3
func (u *S3Uploader) UploadFileToS3(fname string, S3path string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", fname, err)
	}
	// Upload the file to S3.
	_, err = u.S3ManagerUploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(u.Bucket),
		Key:    aws.String(S3path),
		Body:   f,
	})
	if err != nil {
		return err
	}
	return nil
}
