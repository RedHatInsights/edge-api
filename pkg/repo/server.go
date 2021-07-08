package repo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
)

//Server is an interface for a served repository
type Server interface {
	ServeRepo(w http.ResponseWriter, r *http.Request)
}

//FileServer defines the how the files are served for the repository
type FileServer struct {
	BasePath string
}

//ServeRepo provides file serving of the repository
func (s *FileServer) ServeRepo(w http.ResponseWriter, r *http.Request) {
	log.Debugf("FileServer::ServeRepo::r: %#v", r)
	name, pathPrefix, err := getNameAndPrefix(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.BasePath, name)
	log.Debugf("FileServer::ServeRepo::path: %#v", path)
	fs := http.StripPrefix(pathPrefix, http.FileServer(http.Dir(path)))
	fs.ServeHTTP(w, r)
}

//S3Proxy defines the mechanism to proxy data from S3
type S3Proxy struct {
	Client *s3.S3
	Bucket string
}

//NewS3Proxy creates a method to obtain a new S3 proxy
func NewS3Proxy() *S3Proxy {
	cfg := config.Get()
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		// Force enable Shared Config support
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := s3.New(sess)
	if cfg.BucketRegion != "" {
		client.Config.Region = &cfg.BucketRegion
	}
	return &S3Proxy{
		Client: client,
		Bucket: cfg.BucketName,
	}
}

// ServeRepo proxies requests to a backing object storage bucket
// The request is modified from:
//  path/to/api/$name/path/in/repo
// to:
//  bucket/$account/$name/path/in/repo
func (p *S3Proxy) ServeRepo(w http.ResponseWriter, r *http.Request) {
	log.Debugf("S3Proxy::ServeRepo::r: %#v", r)

	_, pathPrefix, err := getNameAndPrefix(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	account := chi.URLParam(r, "account")
	if account == "" {
		account, err = common.GetAccount(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	_r := strings.Index(r.URL.Path, pathPrefix)
	realPath := filepath.Join(account, string(r.URL.Path[_r+len(pathPrefix):]))
	log.Debugf("S3Proxy::ServeRepo::realPath: %#v", realPath)

	o, err := p.Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(realPath),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				log.Debugf("S3Proxy::ServeRepo::s3.ErrCodeNoSuchKey: %#v", realPath)
				err := errors.NewNotFound(fmt.Sprintf("S3 Object %s not found.", realPath))
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
			case s3.ErrCodeInvalidObjectState:
				log.Debugf("S3Proxy::ServeRepo::s3.ErrCodeInvalidObjectState: %#v", realPath)
				err := errors.NewNotFound(fmt.Sprintf("S3 Object %s not found.", realPath))
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
			default:
				err := errors.NewInternalServerError()
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
			}
		} else {
			// log the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Debugf("S3Proxy::ServeRepo::UnhandledS3Error: %#v", err.Error())
		}
		return
	}

	defer o.Body.Close()
	_, err = io.Copy(w, o.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
