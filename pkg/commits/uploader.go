package commits

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

//Uploader is an interface for uploading repository
type Uploader interface {
	UploadRepo(src string, account string) (string, error)
}

//S3Uploader defines the mechanism to upload data to S3
type S3Uploader struct {
	Client            *s3.S3
	S3ManagerUploader *s3manager.Uploader
	Bucket            string
}

// FileUploader isn't actually an uploader but implements the interface in
// order to allow the workflow to be done to completion on a local machine
// without S3
type FileUploader struct {
	BaseDir string
}

// UploadRepo is Basically a dummy function that returns the src, but allows offline
// development without S3 and satisfies the interface
func (u *FileUploader) UploadRepo(src string, account string) (string, error) {
	return src, nil
}

//NewS3Uploader creates a method to obtain a new S3 uploader
func NewS3Uploader() *S3Uploader {
	cfg := config.Get()
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		// Force enable Shared Config support
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := s3.New(sess)
	if cfg.BucketRegion != "" {
		client.Config.Region = &cfg.BucketRegion
	}
	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.Concurrency = 1
	})
	return &S3Uploader{
		Client:            client,
		S3ManagerUploader: uploader,
		Bucket:            cfg.BucketName,
	}
}

// UploadRepo uploads the repo to a backing object storage bucket
// the repository is uploaded to bucket/$account/$name/
func (u *S3Uploader) UploadRepo(src string, account string) (string, error) {
	cfg := config.Get()

	log.Debugf("S3Uploader::UploadRepo::src: %#v", src)
	log.Debugf("S3Uploader::UploadRepo::account: %#v", account)

	// FIXME: might experiment with doing this concurrently but I've read that
	//		  that can get you rate limited by S3 pretty quickly so we'll mess
	//		  with that later.
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warnf("incoming error!: %#v", err)
		}
		log.Debugf("S3Uploader::UploadRepo::path: %#v", path)
		if info.IsDir() {
			return nil
		}

		err = u.UploadFileToS3(path,
			fmt.Sprintf("%s/%s", account, strings.TrimPrefix(path, cfg.UpdateTempPath)),
		)
		if err != nil {
			log.Warnf("error: %v", err)
			return err
		}
		return nil
	})

	region := *u.Client.Config.Region
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/%s", u.Bucket, region, account, strings.TrimPrefix(src, cfg.UpdateTempPath))
	return s3URL, nil
}

// UploadFileToS3 takes a FILename path as a string and then uploads that to
// the supplied location in s3
func (u *S3Uploader) UploadFileToS3(fname string, S3path string) error {
	log.Debugf("S3Uploader::UploadFileToS3::fname: %#v", fname)
	log.Debugf("S3Uploader::UploadFileToS3::S3path: %#v", S3path)
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", fname, err)
	}
	defer f.Close()
	// Upload the file to S3.
	result, err := u.Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(u.Bucket),
		Key:    aws.String(S3path),
		Body:   f,
	})

	log.Debugf("S3Uploader::UploadRepo::result: %#v", result)
	if err != nil {
		return err
	}
	return nil
}
