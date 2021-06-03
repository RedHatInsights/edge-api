package repo

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
)

//uploader is an interface for uploading repository
type Uploader interface {
	UploadRepo(w http.ResponseWriter, r *http.Request)
}

//S3Uploader defines the mechanism to upload data to S3
type S3Uploader struct {
	Client   *s3.S3
	Uploader *s3manager.Uploader
	Bucket   string
}

//NewS3Proxy creates a method to obtain a new S3 proxy
func NewS3Uploader() *S3Uploader {
	cfg := config.Get()
	sess := session.Must(session.NewSession())
	client := s3.New(sess)
	uploader := s3manager.NewUploader(sess)
	return &S3Uploader{
		Client:   client,
		Uploader: uploader,
		Bucket:   cfg.BucketName,
	}
}

// UploadReopo uploads the repo to a backing object storage bucket
// the repository is uploaded to
//  bucket/$account/$name/
func (u *S3Uploader) UploadRepo(w http.ResponseWriter, r *http.Request) {

	name, _, err := getNameAndPrefix(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// FIXME: might experiment with doing this concurrently but I've read that
	//		  that can get you rate limited by S3 pretty quickly so we'll mess
	//		  with that later.
	filepath.Walk(filepath.Join("/tmp", name), func(path string, info os.FileInfo, err error) error {
		err = u.UploadFileToS3(path, filepath.Join(account, "/", string(name)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
		return nil
	})
}

// UploadFileToS3 takes a FILename path as a string and then uploads that to
// the supplied location in s3
func (u *S3Uploader) UploadFileToS3(fname string, S3path string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", fname, err)
	}
	// Upload the file to S3.
	_, err = u.Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(u.Bucket),
		Key:    aws.String(S3path),
		Body:   f,
	})
	if err != nil {
		return err
	}
	return nil
}
