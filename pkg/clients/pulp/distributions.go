package pulp

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (ps *PulpService) DistributionsCreate(ctx context.Context, name, basePath, repoHref, contentGuard string) (*OstreeOstreeDistributionResponse, error) {
	body := OstreeOstreeDistribution{
		Name:         name,
		BasePath:     basePath,
		Repository:   &repoHref,
		ContentGuard: &contentGuard,
	}
	resp, err := ps.cwr.DistributionsOstreeOstreeCreateWithResponse(ctx, ps.dom, body, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON202 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	hrefs, err := ps.WaitForTask(ctx, resp.JSON202.Task)
	if err != nil {
		return nil, err
	}
	if len(hrefs) != 1 {
		return nil, fmt.Errorf("unexpected number of created resources: %d", len(hrefs))
	}
	href := hrefs[0]

	result, err := ps.DistributionsRead(ctx, ScanUUID(&href))
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (ps *PulpService) DistributionsRead(ctx context.Context, id uuid.UUID) (*OstreeOstreeDistributionResponse, error) {
	req := DistributionsOstreeOstreeReadParams{}
	resp, err := ps.cwr.DistributionsOstreeOstreeReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (ps *PulpService) DistributionsDelete(ctx context.Context, id uuid.UUID) error {
	resp, err := ps.cwr.DistributionsOstreeOstreeDelete(ctx, ps.dom, id, addAuthenticationHeader)

	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}

	return nil
}
