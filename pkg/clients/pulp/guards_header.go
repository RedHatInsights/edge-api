package pulp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/ptr"
	"github.com/sirupsen/logrus"
)

const JQOrgID = ".identity.org_id"

func headerGuardName(orgID string) string {
	return "EDGE_HEADER_ORGID=" + orgID
}

// HeaderGuardCreate creates a new header guard for the given orgID.
func (ps *PulpService) HeaderGuardCreate(ctx context.Context, orgID string) (*HeaderContentGuardResponse, error) {
	req := ContentguardsCoreHeaderCreateJSONRequestBody{
		Name:        headerGuardName(orgID),
		HeaderName:  "x-rh-identity",
		HeaderValue: orgID,
		JqFilter:    ptr.To(JQOrgID),
		Description: ptr.To("EDGE"),
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

// HeaderGuardRead returns the header guard with the given ID.
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

// HeaderGuardReadByOrgID returns the header guard for the given orgID.
func (ps *PulpService) HeaderGuardReadByOrgID(ctx context.Context, orgID string) (*HeaderContentGuardResponse, error) {
	hgl, err := ps.HeaderGuardList(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if len(hgl) == 0 {
		return nil, fmt.Errorf("header guard not found for orgID %s: %w", orgID, ErrRecordNotFound)
	}

	id := ScanUUID(hgl[0].PulpHref)
	return ps.HeaderGuardRead(ctx, id)
}

// HeaderGuardEnsure ensures that the header guard is created and returns it. The method is idempotent.
func (ps *PulpService) HeaderGuardEnsure(ctx context.Context, orgID string) (*HeaderContentGuardResponse, error) {
	cg, err := ps.HeaderGuardReadByOrgID(ctx, headerGuardName(orgID))
	// nolint: gocritic
	if errors.Is(err, ErrRecordNotFound) {
		cg, err = ps.HeaderGuardCreate(ctx, orgID)
		if err != nil {
			return nil, err
		}

		return cg, nil
	} else if err != nil {
		return nil, err
	} else if cg == nil {
		return nil, fmt.Errorf("unexpected nil guard")
	}

	if cg.JqFilter == nil || *cg.JqFilter != JQOrgID {
		logrus.WithContext(ctx).Warnf("unexpected Header Content Guard: %s, expected: %s, deleting it", ptr.FromOrEmpty(cg.JqFilter), JQOrgID)
		err = ps.HeaderGuardDelete(ctx, ScanUUID(cg.PulpHref))
		if err != nil {
			return nil, err
		}

		cg, err = ps.HeaderGuardCreate(ctx, orgID)
		if err != nil {
			return nil, err
		}
	}
	return cg, nil
}

// HeaderGuardDelete deletes the header guard with the given ID.
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

// HeaderGuardList returns a list of header guards with the given name filter.
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

	if resp.JSON200.Count > DefaultPageSize {
		return nil, fmt.Errorf("default page size too small: %d", resp.JSON200.Count)
	}

	return resp.JSON200.Results, nil
}
