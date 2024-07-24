package pulp

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/config"
)

func (ps *PulpService) DomainsCreate(ctx context.Context, domain string) (*DomainResponse, error) {
	req := DomainsCreateJSONRequestBody{
		Name:         domain,
		StorageClass: StoragesBackendsS3boto3S3Boto3Storage,
		StorageSettings: map[string]interface{}{
			"default_acl": "private",
			"region_name": config.Get().PulpS3Region,
			"access_key":  config.Get().PulpS3AccessKey,
			"secret_key":  config.Get().PulpS3SecretKey,
			"bucket_name": config.Get().PulpS3BucketName,
		},
	}
	resp, err := ps.cwr.DomainsCreateWithResponse(ctx, ps.dom, req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON201, nil
}

func (ps *PulpService) DomainsRead(ctx context.Context, domain string, id uuid.UUID) (*DomainResponse, error) {
	req := DomainsReadParams{}
	resp, err := ps.cwr.DomainsReadWithResponse(ctx, domain, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (ps *PulpService) DomainsDelete(ctx context.Context, domain string, id uuid.UUID) (*AsyncOperationResponse, error) {
	resp, err := ps.cwr.DomainsDeleteWithResponse(ctx, domain, id, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON202 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON202, nil
}

func (ps *PulpService) DomainsList(ctx context.Context, nameFilter string) ([]DomainResponse, error) {
	req := DomainsListParams{
		Limit: &DefaultPageSize,
	}
	if nameFilter != "" {
		req.Name = &nameFilter
	}

	resp, err := ps.cwr.DomainsListWithResponse(ctx, ps.dom, &req, addAuthenticationHeader)

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
