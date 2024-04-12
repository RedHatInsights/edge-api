// FIXME: golangci-lint
// nolint:govet,revive
package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// Uploader is an interface for uploading repository
type Uploader interface {
	UploadRepo(src string, account string, acl string) (string, error)
	UploadFile(fname string, uploadPath string) (string, error)
}

// NewUploader returns the uploader used by EdgeAPI based on configurations
func NewUploader(log log.FieldLogger) Uploader {
	cfg := config.Get()
	var uploader Uploader
	uploader = &LocalUploader{
		BaseDir: "/tmp",
		log:     log,
	}
	if !cfg.Local {
		uploader = NewS3Uploader(log, GetNewS3Client())
	}
	return uploader
}

// S3Uploader defines the mechanism to upload data to S3
type S3Uploader struct {
	Client S3ClientInterface
	Bucket string
	log    log.FieldLogger
}

// LocalUploader isn't actually an uploader but implements the interface in
// order to allow the workflow to be done to completion on a local machine
// without S3
type LocalUploader struct {
	BaseDir string
	log     log.FieldLogger
}

// UploadRepo just returns the src repo folder
// It doesnt do anything and it doesn't delete the original folder
// It returns error if the repo is not using u.BaseDir as its base folder
// Allowing offline development without S3 and satisfying the interface
func (u *LocalUploader) UploadRepo(src string, account string, acl string) (string, error) {
	if strings.HasPrefix(src, u.BaseDir) {
		return src, nil
	}
	return "", fmt.Errorf("invalid folder to upload on local uploader")
}

// UploadFile basically copies a file to the local server path
// Allowing offline development without S3 and satisfying the interface
func (u *LocalUploader) UploadFile(fname string, uploadPath string) (string, error) {
	destfile := filepath.Clean(u.BaseDir + "/" + uploadPath)
	u.log.WithFields(log.Fields{"fname": fname, "destfine": destfile}).Debug("Copying fname to destfile")
	cmd := exec.Command("cp", fname, destfile) //#nosec G204 - This uploadPath variable is actually controlled by the calling method
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return destfile, nil
}

// NewS3Uploader return a new S3Uploader
func NewS3Uploader(log log.FieldLogger, client S3ClientInterface) *S3Uploader {
	cfg := config.Get()
	return &S3Uploader{
		Client: client,
		Bucket: cfg.BucketName,
		log:    log,
	}
}

// Struct that contains all details required to upload a file to a destination
type uploadDetails struct {
	fileName   string
	uploadPath string
	uploader   *S3Uploader
	done       chan bool
	count      int
}

func (u *S3Uploader) worker(uploadQueue chan *uploadDetails, acl string, cfg *config.EdgeConfig) {
	retryDelay := time.Duration(cfg.RepoFileUploadDelay)
	for p := range uploadQueue {
		// attempt to upload a file multiple times before erroring
		for attempt := 1; attempt <= int(cfg.RepoFileUploadAttempts); attempt++ {
			fname, err := p.uploader.UploadFileWithACL(p.fileName, p.uploadPath, acl)
			// log on file upload failure and retry
			if err != nil {
				u.log.WithFields(log.Fields{"fname": fname, "attempt": attempt, "count": p.count, "error": err.Error()}).Error("Error uploading file")
				time.Sleep(retryDelay * time.Second)
				continue
			}
			// if upload succeeds on retry, log Info to show error is resolved, else use Trace on first try
			if attempt > 1 {
				u.log.WithFields(log.Fields{"fname": fname, "attempt": attempt, "count": p.count}).Info("File was uploaded successfully")
			} else {
				u.log.WithFields(log.Fields{"fname": fname, "attempt": attempt, "count": p.count}).Trace("File was uploaded successfully")
			}

			break
		}

		p.done <- true
	}
}

// UploadRepo uploads the repo to a backing object storage bucket
// the repository is uploaded to bucket/$account/$name/ with ACL "private" or "public-read"
func (u *S3Uploader) UploadRepo(src string, account string, acl string) (string, error) {
	cfg := config.Get()

	if acl == "" {
		acl = "private"
	}

	u.log = u.log.WithFields(log.Fields{"src": src, "account": account})
	u.log.Info("Uploading repo")
	// Wait group is created per request
	// this allows multiple repo's to be independently uploaded simultaneously
	count := 0

	var uploadDetailsList []*uploadDetails

	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			u.log.WithField("error", err.Error()).Error("Error opening file")
		}
		if info.IsDir() {
			return nil
		}

		res := new(uploadDetails)
		res.fileName = path
		res.uploadPath = fmt.Sprintf("%s/%s", account, strings.TrimPrefix(path, cfg.RepoTempPath))
		res.uploader = u
		res.count = count
		res.done = make(chan bool)
		uploadDetailsList = append(uploadDetailsList, res)
		count++
		return nil
	}); err != nil {
		u.log.WithField("error", err.Error()).Error("Error walking directory")
		return "", err
	}

	log.WithField("fileCount", len(uploadDetailsList)).Debug("Files are being uploaded....")

	uploadQueue := make(chan *uploadDetails, len(uploadDetailsList))
	for _, u := range uploadDetailsList {
		uploadQueue <- u
	}

	numberOfWorkers := cfg.UploadWorkers
	for i := 0; i < numberOfWorkers; i++ {
		go u.worker(uploadQueue, acl, cfg)
	}

	for i, ud := range uploadDetailsList {
		<-ud.done
		u.log.WithField("index", i).Trace("File is done")
		close(ud.done)
	}
	close(uploadQueue)
	u.log.Debug("Channel is closed...")
	region := config.Get().BucketRegion
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/%s", u.Bucket, region, account, strings.TrimPrefix(src, cfg.RepoTempPath))
	u.log.WithField("s3URL", s3URL).Info("Files are done uploading...")
	return s3URL, nil
}

// UploadFileWithACL  upload a file from local file system to remote s3 bucket location using the acl supplied.
// uploadFile takes a Filename path as a string and then uploads that to
func (u *S3Uploader) UploadFileWithACL(fname string, uploadPath string, acl string) (string, error) {
	f, err := os.Open(filepath.Clean(fname))
	if err != nil {
		return "", fmt.Errorf("failed to open file %q, %v", fname, err)
	}
	if acl == "" {
		acl = "private"
	}

	// Upload the file to S3.
	_, err = u.Client.PutObject(f, u.Bucket, uploadPath, acl)

	if err != nil {
		u.log.WithField("error", err.Error()).Error("Error uploading to AWS S3")
		return "", err
	}
	if err := f.Close(); err != nil {
		u.log.WithField("error", err.Error()).Error("Error closing file")
		return "", err
	}
	region := config.Get().BucketRegion
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", u.Bucket, region, uploadPath)
	return s3URL, nil
}

// UploadFile takes a Filename path as a string and then uploads that to the supplied location in s3
func (u *S3Uploader) UploadFile(fname string, uploadPath string) (string, error) {
	return u.UploadFileWithACL(fname, uploadPath, "private")
}
