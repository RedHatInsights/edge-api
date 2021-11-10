package files

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
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
func NewUploader() Uploader {
	cfg := config.Get()
	var uploader Uploader
	uploader = &FileUploader{
		BaseDir: "./",
	}
	if cfg.BucketName != "" {
		uploader = newS3Uploader()
	}
	return uploader
}

// S3Uploader defines the mechanism to upload data to S3
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

// UploadFile is Basically a dummy function that returns no error but allows offline
// development without S3 and satisfies the interface
func (u *FileUploader) UploadFile(fname string, uploadPath string) (string, error) {
	return fname, nil
}

func newS3Uploader() *S3Uploader {
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

func worker(uploadQueue chan *uploadDetails) {
	for p := range uploadQueue {
		fname, err := p.uploader.UploadFile(p.fileName, p.uploadPath)
		log.Debugf("Filename: %s with counter %d was uploaded sucessfully", fname, p.count)
		if err != nil {
			log.Errorf("error: %v", err)
		}
		p.done <- true
		log.Debugf("Filename: %s with counter %d was done uploading", fname, p.count)
	}
}

// UploadRepo uploads the repo to a backing object storage bucket
// the repository is uploaded to bucket/$account/$name/
func (u *S3Uploader) UploadRepo(src string, account string) (string, error) {
	cfg := config.Get()

	log.Debugf("S3Uploader::UploadRepo::src: %#v", src)
	log.Debugf("S3Uploader::UploadRepo::account: %#v", account)

	//Wait group is created per request
	//this allows multiple repo's to be independently uploaded simultaneously
	count := 0

	var uploadDetailsList []*uploadDetails

	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warnf("incoming error!: %#v", err)
		}
		log.Debugf("S3Uploader::UploadRepo::path: %#v", path)
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
	})

	log.Infof("Files are being uploaded.... %d files to upload", len(uploadDetailsList))

	uploadQueue := make(chan *uploadDetails, len(uploadDetailsList))
	for _, u := range uploadDetailsList {
		uploadQueue <- u
	}

	numberOfWorkers := cfg.UploadWorkers
	for i := 0; i < numberOfWorkers; i++ {
		go worker(uploadQueue)
	}

	for i, u := range uploadDetailsList {
		<-u.done
		log.Debugf("%d file is done", i)
		close(u.done)
	}
	log.Infof("Files are done uploading...")
	close(uploadQueue)
	log.Infof("Channel is closed...")
	tarFile(src)
	_, error := u.UploadFile(filepath.Join(src, "repo.tar"), fmt.Sprintf("%s/%s", account, strings.TrimPrefix(src, cfg.RepoTempPath)))
	if error != nil {
		log.Error("Error on tar upload...")
	}
	region := *u.Client.Config.Region
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/%s", u.Bucket, region, account, strings.TrimPrefix(src, cfg.RepoTempPath))
	return s3URL, nil
}

// UploadFile takes a Filename path as a string and then uploads that to
// the supplied location in s3
func (u *S3Uploader) UploadFile(fname string, uploadPath string) (string, error) {
	log.Debugf("S3Uploader::UploadFileToS3::fname: %#v", fname)
	log.Debugf("S3Uploader::UploadFileToS3::S3path: %#v", uploadPath)
	f, err := os.Open(fname)
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

	log.Debugf("S3Uploader::UploadRepo::result: %#v", result)
	if err != nil {
		return "", err
	}
	f.Close()
	region := *u.Client.Config.Region
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", u.Bucket, region, uploadPath)
	return s3URL, nil
}

func tarFile(source string) error {
	log.Debugf("\n:::: tarFile::source:::: %#v\n", source)
	filename := filepath.Base(source)
	target := filepath.Join(source, fmt.Sprintf("%s.tar", filename))
	log.Debugf("\n:::: tarFile::target:::: %#v\n", target)
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	tarball := tar.NewWriter(tarfile)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if baseDir != "" {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
			}

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})
}
