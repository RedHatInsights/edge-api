package repo

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
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
	name, pathPrefix, err := getNameAndPrefix(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.BasePath, name)
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
	sess := session.Must(session.NewSession())
	client := s3.New(sess)
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

	_, pathPrefix, err := getNameAndPrefix(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_r := strings.Index(r.URL.Path, pathPrefix)
	realPath := filepath.Join(account, string(r.URL.Path[_r+len(pathPrefix):]))

	o, err := p.Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(realPath),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer o.Body.Close()
	_, err = io.Copy(w, o.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
