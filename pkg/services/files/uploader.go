package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

//Uploader is an interface for uploading repository
type Uploader interface {
	UploadRepo(src string, account string) (string, error)
	UploadFile(fname string, uploadPath string) (string, error)
}

// NewUploader returns the uploader used by EdgeAPI based on configurations
func NewUploader(log *log.Entry) Uploader {
	cfg := config.Get()
	var uploader Uploader
	uploader = &FileUploader{
		BaseDir: "/tmp",
	}
	if !cfg.Local {
		uploader = newS3Uploader(log)
	}
	return uploader
}

// S3Uploader defines the mechanism to upload data to S3
type S3Uploader struct {
	Client            *s3.S3
	S3ManagerUploader *s3manager.Uploader
	Bucket            string
	log               *log.Entry
}

// FileUploader isn't actually an uploader but implements the interface in
// order to allow the workflow to be done to completion on a local machine
// without S3
type FileUploader struct {
	BaseDir string
}

// UploadRepo basically copies everything on the src to the local server path
// Allowing offline development without S3 and satisfying the interface
func (u *FileUploader) UploadRepo(src string, account string) (string, error) {
	return src, nil
}

// UploadFile is Basically a dummy function that returns no error but allows offline
// development without S3 and satisfies the interface
func (u *FileUploader) UploadFile(fname string, uploadPath string) (string, error) {
	destfile := u.BaseDir + "image.iso"
	cmd := exec.Command("cp", fname, destfile)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return destfile, nil
}

func newS3Uploader(log *log.Entry) *S3Uploader {
	cfg := config.Get()
	var sess *session.Session
	if cfg.Debug {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			// Force enable Shared Config support
			SharedConfigState: session.SharedConfigEnable,
		}))
	} else {
		var err error
		sess, err = session.NewSession(&aws.Config{
			Region:      cfg.BucketRegion,
			Credentials: credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		})
		if err != nil {
			panic(err)
		}
	}
	client := s3.New(sess)
	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.Concurrency = 1
	})
	return &S3Uploader{
		Client:            client,
		S3ManagerUploader: uploader,
		Bucket:            cfg.BucketName,
		log:               log,
	}
}

//Struct that contains all details required to upload a file to a destination
type uploadDetails struct {
	fileName   string
	uploadPath string
	uploader   *S3Uploader
	done       chan bool
	count      int
}

func (u *S3Uploader) worker(uploadQueue chan *uploadDetails) {
	for p := range uploadQueue {
		fname, err := p.uploader.UploadFile(p.fileName, p.uploadPath)
		if err != nil {
			u.log.WithFields(log.Fields{"fname": fname, "count": p.count, "error": err.Error()}).Error("Error uploading file")
		}
		u.log.WithFields(log.Fields{"fname": fname, "count": p.count}).Trace("File was uploaded successfully")
		p.done <- true
	}
}

// UploadRepo uploads the repo to a backing object storage bucket
// the repository is uploaded to bucket/$account/$name/
func (u *S3Uploader) UploadRepo(src string, account string) (string, error) {
	cfg := config.Get()

	u.log = u.log.WithFields(log.Fields{"src": src, "account": account})
	u.log.Info("Uploading repo")
	//Wait group is created per request
	//this allows multiple repo's to be independently uploaded simultaneously
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
		go u.worker(uploadQueue)
	}

	for i, ud := range uploadDetailsList {
		<-ud.done
		u.log.WithField("index", i).Trace("File is done")
		close(ud.done)
	}
	close(uploadQueue)
	u.log.Debug("Channel is closed...")
	region := *u.Client.Config.Region
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/%s", u.Bucket, region, account, strings.TrimPrefix(src, cfg.RepoTempPath))
	u.log.WithField("s3URL", s3URL).Info("Files are done uploading...")
	return s3URL, nil
}

// UploadFile takes a Filename path as a string and then uploads that to
// the supplied location in s3
func (u *S3Uploader) UploadFile(fname string, uploadPath string) (string, error) {
	u.log = u.log.WithFields(log.Fields{"fname": fname, "uploadPath": uploadPath})
	u.log.Info("Uploading file")
	f, err := os.Open(filepath.Clean(fname))
	if err != nil {
		return "", fmt.Errorf("failed to open file %q, %v", fname, err)
	}
	// Upload the file to S3.
	result, err := u.Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(u.Bucket),
		Key:    aws.String(uploadPath),
		Body:   f,
		ACL:    aws.String("public-read"),
	})

	u.log.WithField("result", result).Info("Finished upload to AWS S3")
	if err != nil {
		u.log.WithField("error", err.Error()).Error("Error uploading to AWS S3")
		return "", err
	}
	if err := f.Close(); err != nil {
		u.log.WithField("error", err.Error()).Error("Error closing file")
		return "", err
	}
	region := *u.Client.Config.Region
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", u.Bucket, region, uploadPath)
	u.log.WithField("s3URL", s3URL).Info("Upload file finished...")
	return s3URL, nil
}
