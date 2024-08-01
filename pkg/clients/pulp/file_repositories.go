package pulp

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/ptr"
)

const FileRepositoryName = "edge-tar-uploads"

// FileRepositoriesCreate creates a new file repository.
func (ps *PulpService) FileRepositoriesCreate(ctx context.Context, name string) (*FileFileRepositoryResponse, error) {
	body := FileFileRepository{
		Name:               name,
		Description:        ptr.To("Temporary repository for file uploads"),
		RetainRepoVersions: ptr.To(int64(1)),
	}
	resp, err := ps.cwr.RepositoriesFileFileCreateWithResponse(ctx, ps.dom, body, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON201, nil
}

// FileRepositoriesImport imports a file into a file repository. Returns the artifact href and a version
// artifact href which can be used to delete the artifact.
func (ps *PulpService) FileRepositoriesImport(ctx context.Context, repoHref, url string) (string, string, error) {
	body := ContentFileFilesCreateFormdataRequestBody{
		Repository:   &repoHref,
		FileUrl:      &url, // Pulp 3.56.1 or higher needed for this flag
		RelativePath: uuid.NewString(),
	}
	resp, err := ps.cwr.ContentFileFilesCreateWithFormdataBody(ctx, ps.dom, body, addAuthenticationHeader)
	defer func() { _ = resp.Body.Close() }()

	if err != nil {
		return "", "", err
	}

	if resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode, body)
	}

	cfcr, err := ParseContentFileFilesCreateResponse(resp)
	if err != nil {
		return "", "", err
	}

	hrefs, err := ps.WaitForTask(ctx, cfcr.JSON202.Task)
	if err != nil {
		return "", "", err
	}

	artifactHref := ""
	versionHref := ""
	for _, h := range hrefs {
		// pulp returns several resources, we are interested in the file resource
		if strings.Contains(h, "content/file/files") {
			file, err := ps.FileRepositoriesArtifactRead(ctx, ScanUUID(ptr.To(h)))
			if err != nil {
				return "", "", err
			}
			artifactHref = *file.Artifact
		}
		if strings.Contains(h, "repositories/file/file") {
			versionHref = h
		}
	}

	return artifactHref, versionHref, nil
}

// FileRepositoriesArtifactRead reads a file artifact from a file repository.
func (ps *PulpService) FileRepositoriesArtifactRead(ctx context.Context, id uuid.UUID) (*FileFileContentResponse, error) {
	req := ContentFileFilesReadParams{}
	resp, err := ps.cwr.ContentFileFilesReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

// FileRepositoriesVersionDelete deletes a file version from a file repository.
func (ps *PulpService) FileRepositoriesVersionDelete(ctx context.Context, id uuid.UUID, version int64) error {
	resp, err := ps.cwr.RepositoriesFileFileVersionsDelete(ctx, ps.dom, id, version, addAuthenticationHeader)

	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 202 {
		return fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}

	return nil
}

// FileRepositoriesRead reads a file repository.
func (ps *PulpService) FileRepositoriesRead(ctx context.Context, id uuid.UUID) (*FileFileRepositoryResponse, error) {
	req := RepositoriesFileFileReadParams{}
	resp, err := ps.cwr.RepositoriesFileFileReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

// FileRepositoriesDelete deletes a file repository.
func (ps *PulpService) FileRepositoriesDelete(ctx context.Context, id uuid.UUID) error {
	resp, err := ps.cwr.RepositoriesFileFileDelete(ctx, ps.dom, id, addAuthenticationHeader)

	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}

	return nil
}

// FileRepositoriesList lists file repositories.
func (ps *PulpService) FileRepositoriesList(ctx context.Context, nameFilter string) ([]FileFileRepositoryResponse, error) {
	req := RepositoriesFileFileListParams{
		Limit: &DefaultPageSize,
	}
	if nameFilter != "" {
		req.Name = &nameFilter
	}

	resp, err := ps.cwr.RepositoriesFileFileListWithResponse(ctx, ps.dom, &req, addAuthenticationHeader)

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

// FileRepositoriesEnsure ensures a file repository exists.
func (ps *PulpService) FileRepositoriesEnsure(ctx context.Context) (string, error) {
	var repo *FileFileRepositoryResponse
	repos, err := ps.FileRepositoriesList(ctx, FileRepositoryName)
	if err != nil {
		return "", err
	}

	if len(repos) != 1 {
		repo, err = ps.FileRepositoriesCreate(ctx, FileRepositoryName)
		if err != nil {
			return "", err
		}
	} else {
		repo = &repos[0]
	}

	return *repo.PulpHref, nil
}
