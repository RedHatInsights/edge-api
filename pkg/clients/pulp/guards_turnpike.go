// Package pulp implements routines for Edge API to interact with the Pulp OSTree service.
//
// The Turnpike Content Guard enables cert-based communication between services and the Pulp service.
package pulp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/ptr"
)

const TurnpikeGuardName = "ostree_turnpike_guard"
const TurnpikeJQFilter = ".identity.x509.subject_dn"

// TurnpikeGuardList returns a list of header guards. The nameFilter can be used to filter the results.
func (ps *PulpService) TurnpikeGuardList(ctx context.Context, nameFilter string) ([]HeaderContentGuardResponse, error) {
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

	if resp.JSON200.Count == 0 || resp.JSON200.Results[0].PulpHref == nil {
		return nil, ErrRecordNotFound
	}

	return resp.JSON200.Results, nil
}

// TurnpikeGuardRead returns the Turnpike guard with the given ID.
func (ps *PulpService) TurnpikeGuardRead(ctx context.Context, id uuid.UUID) (*HeaderContentGuardResponse, error) {
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

// TurnpikeGuardFind returns the Turnpike guard.
func (ps *PulpService) TurnpikeGuardFind(ctx context.Context) (*HeaderContentGuardResponse, error) {
	hgl, err := ps.TurnpikeGuardList(ctx, TurnpikeGuardName)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"name":  TurnpikeGuardName,
			"error": err.Error(),
		}).Error("Turnpike content guard not found")

		if errors.Is(err, ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}

		return nil, err
	}

	id := ScanUUID(hgl[0].PulpHref)
	return ps.TurnpikeGuardRead(ctx, id)
}

// TurnpikeGuardCreate creates a new Turnpike guard and returns it.
func (ps *PulpService) TurnpikeGuardCreate(ctx context.Context) (*HeaderContentGuardResponse, error) {
	cfg := config.Get()

	if cfg.Pulp.GuardSubjectDN == "" {
		return nil, fmt.Errorf("guard subject dn is not set")
	}

	req := HeaderContentGuard{
		Name:        TurnpikeGuardName,
		Description: ptr.To("EDGE"),
		HeaderName:  "x-rh-identity",
		HeaderValue: cfg.Pulp.GuardSubjectDN,
		JqFilter:    ptr.To(TurnpikeJQFilter),
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

// TurnpikeGuardEnsure ensures that the Turnpike guard is created and returns it. The method is idempotent.
func (ps *PulpService) TurnpikeGuardEnsure(ctx context.Context) (*HeaderContentGuardResponse, error) {
	cg, err := ps.TurnpikeGuardFind(ctx)
	if errors.Is(err, ErrRecordNotFound) {
		// turnpike guard is not found, so create one
		cg, err = ps.TurnpikeGuardCreate(ctx)
		if err != nil {
			return nil, err
		}

		return cg, nil
	}
	if err != nil {
		return nil, err
	}
	if cg == nil {
		return nil, fmt.Errorf("unexpected nil guard")
	}

	return cg, nil
}

// TurnpikeGuardDelete deletes the Turnpike guard with the given ID.
func (ps *PulpService) TurnpikeGuardDelete(ctx context.Context, id uuid.UUID) error {
	listParams := ContentguardsCoreHeaderListRolesParams{}
	roles, err := ps.cwr.ContentguardsCoreHeaderListRolesWithResponse(ctx, ps.dom, id, &listParams, addAuthenticationHeader)

	if err != nil {
		return err
	}

	if roles.JSON200 == nil {
		return fmt.Errorf("unexpected response: %d, body: %s", roles.StatusCode(), string(roles.Body))
	}

	for _, role := range roles.JSON200.Roles {
		nr := NestedRole{
			Role:  role.Role,
			Users: &[]string{},
		}

		logrus.WithContext(ctx).Warnf("removing Turnpike guardrole: %s", role.Role)
		removed, err := ps.cwr.ContentguardsCoreHeaderRemoveRoleWithResponse(ctx, ps.dom, id, nr, addAuthenticationHeader)

		if err != nil {
			return err
		}

		if removed.JSON201 == nil {
			return fmt.Errorf("unexpected response: %d, body: %s", removed.StatusCode(), string(removed.Body))
		}
	}

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
