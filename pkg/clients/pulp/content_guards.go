package pulp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const JQOrgID = ".identity.org_id"

func formatName(name string) string {
	return "ORGID=" + name
}

func (ps *PulpService) HeaderGuardCreate(ctx context.Context, jq, orgID string) (*HeaderContentGuardResponse, error) {
	req := ContentguardsCoreHeaderCreateJSONRequestBody{
		Name:        formatName(orgID),
		HeaderName:  "x-rh-identity",
		HeaderValue: orgID,
		JqFilter:    &jq,
	}
	resp, err := ps.cwr.ContentguardsCoreHeaderCreateWithResponse(ctx, ps.dom, req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON201, nil
}

func (ps *PulpService) HeaderGuardRead(ctx context.Context, id uuid.UUID) (*HeaderContentGuardResponse, error) {
	req := ContentguardsCoreHeaderReadParams{}
	resp, err := ps.cwr.ContentguardsCoreHeaderReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (ps *PulpService) HeaderGuardReadByOrgID(ctx context.Context, orgID string) (*HeaderContentGuardResponse, error) {
	hgl, err := ps.HeaderGuardList(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if len(hgl) == 0 {
		return nil, fmt.Errorf("header guard not found for orgID %s: %w", orgID, ErrRecordNotFound)
	}

	id := ScanUUID(*hgl[0].PulpHref)
	return ps.HeaderGuardRead(ctx, id)
}

func (ps *PulpService) HeaderGuardReadOrCreate(ctx context.Context, jq, orgID string) (*HeaderContentGuardResponse, error) {
	hg, err := ps.HeaderGuardReadByOrgID(ctx, formatName(orgID))
	if errors.Is(err, ErrRecordNotFound) {
		hg, err = ps.HeaderGuardCreate(ctx, jq, orgID)
		if err != nil {
			return nil, err
		}

		return hg, nil
	} else if err != nil {
		return nil, err
	}

	return hg, nil
}

func (ps *PulpService) HeaderGuardDelete(ctx context.Context, id uuid.UUID) error {
	resp, err := ps.cwr.ContentguardsCoreHeaderDelete(ctx, ps.dom, id, addAuthenticationHeader)

	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}

	return nil
}

func (ps *PulpService) HeaderGuardList(ctx context.Context, nameFilter string) ([]HeaderContentGuardResponse, error) {
	req := ContentguardsCoreHeaderListParams{
		Limit: &DefaultPageSize,
	}
	if nameFilter != "" {
		req.Name = &nameFilter
	}

	resp, err := ps.cwr.ContentguardsCoreHeaderListWithResponse(ctx, ps.dom, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200.Count >= DefaultPageSize {
		return nil, fmt.Errorf("default page size too small: %d", resp.JSON200.Count)
	}

	return resp.JSON200.Results, nil
}
