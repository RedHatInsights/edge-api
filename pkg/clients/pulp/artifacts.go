package pulp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func writeMultipart(writer *multipart.Writer, size int64, sha256, filename string, file io.Reader) (int64, error) {
	err := writer.WriteField("size", fmt.Sprintf("%d", size))
	if err != nil {
		return 0, err
	}

	err = writer.WriteField("sha256", sha256)
	if err != nil {
		return 0, err
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return 0, err
	}

	written, err := io.Copy(part, file)
	if err != nil {
		return 0, err
	}

	return written, nil
}

// ArtifactsCreatePipe creates an artifact from a file using a pipe to avoid buffering the file in memory.
// Pulp currently does require Content-Length header, therefore this function figures out length of the stream
// in advance.
func (ps *PulpService) ArtifactsCreatePipe(ctx context.Context, filename string) (*ArtifactResponse, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	sha256h := hash.Sum(nil)
	sha256 := hex.EncodeToString(sha256h)
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// Try to read it first
	list, err := ps.ArtifactsList(ctx, hex.EncodeToString(sha256h))
	if err != nil {
		return nil, err
	}

	if len(list) == 1 {
		return &list[0], nil
	}

	// Write once with zero-sized file to determine the size of the multipart
	sizeBuf := &bytes.Buffer{}
	sizeWriter := multipart.NewWriter(sizeBuf)
	_, err = writeMultipart(sizeWriter, stat.Size(), sha256, filepath.Base(filename), strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	err = sizeWriter.Close()
	if err != nil {
		return nil, err
	}
	var setContentLength = func(_ context.Context, req *http.Request) error {
		req.ContentLength = stat.Size() + int64(sizeBuf.Len())
		return nil
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write through pipe to avoid buffering the file in memory. The total amount of memory used is
	// about 32kB which is the small buffer io.Copy uses.
	var writeErr error
	var written int64
	go func() {
		defer file.Close()
		defer pw.Close()
		defer writer.Close()

		written, writeErr = writeMultipart(writer, stat.Size(), sha256, filepath.Base(filename), file)
	}()

	resp, err := ps.cwr.ArtifactsCreateWithBodyWithResponse(ctx, ps.dom, writer.FormDataContentType(),
		pr, addAuthenticationHeader, setContentLength)

	if err != nil || writeErr != nil {
		// cannot use errors.Wrap because we are not on Go 1.21 yet
		return nil, fmt.Errorf("error creating artifact (bytes written/size: %d/%d): %w (writer err: %s)", written, stat.Size(), err, writeErr)
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, correlation id: %s, bytes written/size: %d/%d, body: %s",
			resp.StatusCode(), resp.HTTPResponse.Header.Get("Correlation-ID"), written, stat.Size(), string(resp.Body))
	}

	return resp.JSON201, nil
}

var ErrArtifactExists = errors.New("artifact already exists")

// ArtifactsCreate creates an artifact from a file.
func (ps *PulpService) ArtifactsCreate(ctx context.Context, filename string) (*ArtifactResponse, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	sha256h := hash.Sum(nil)

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// try to read it first
	list, err := ps.ArtifactsList(ctx, hex.EncodeToString(sha256h))
	if err != nil {
		return nil, err
	}

	if len(list) == 1 {
		return &list[0], nil
	}

	// does not exists, upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	err = writer.WriteField("size", fmt.Sprintf("%d", stat.Size()))
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("sha256", hex.EncodeToString(sha256h))
	if err != nil {
		return nil, err
	}

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	resp, err := ps.cwr.ArtifactsCreateWithBodyWithResponse(ctx, ps.dom, writer.FormDataContentType(), body, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == 400 {
		return nil, ErrArtifactExists
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, correlation id: %s, body: %s", resp.StatusCode(), resp.HTTPResponse.Header.Get("Correlation-ID"), string(resp.Body))
	}

	return resp.JSON201, nil
}

func (ps *PulpService) ArtifactsRead(ctx context.Context, id uuid.UUID) (*ArtifactResponse, error) {
	req := ArtifactsReadParams{}
	resp, err := ps.cwr.ArtifactsReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (ps *PulpService) ArtifactsList(ctx context.Context, sha256 string) ([]ArtifactResponse, error) {
	req := ArtifactsListParams{
		Limit: &DefaultPageSize,
	}
	if sha256 != "" {
		req.Sha256 = &sha256
	}

	resp, err := ps.cwr.ArtifactsListWithResponse(ctx, ps.dom, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200.Count > DefaultPageSize {
		return nil, fmt.Errorf("default page size too small: %d", resp.JSON200.Count)
	}

	return resp.JSON200.Results, nil
}
