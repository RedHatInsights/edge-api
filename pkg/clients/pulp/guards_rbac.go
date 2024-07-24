package pulp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/ptr"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var guardRbacName = "ROLE=readonly"

func rbacUsername() string {
	return config.Get().PulpContentUsername
}

// RbacGuardList returns a list of RBAC guards. The nameFilter can be used to filter the results.
func (ps *PulpService) RbacGuardList(ctx context.Context, nameFilter string) ([]RBACContentGuardResponse, error) {
	req := ContentguardsCoreRbacListParams{
		Limit: &DefaultPageSize,
	}
	if nameFilter != "" {
		req.Name = &nameFilter
	}

	resp, err := ps.cwr.ContentguardsCoreRbacListWithResponse(ctx, ps.dom, &req, addAuthenticationHeader)

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

// RbacGuardRead returns the RBAC guard with the given ID.
func (ps *PulpService) RbacGuardRead(ctx context.Context, id uuid.UUID) (*RBACContentGuardResponse, error) {
	req := ContentguardsCoreRbacReadParams{}
	resp, err := ps.cwr.ContentguardsCoreRbacReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

// RbacGuardFind returns the RBAC guard.
func (ps *PulpService) RbacGuardFind(ctx context.Context) (*RBACContentGuardResponse, error) {
	hgl, err := ps.RbacGuardList(ctx, guardRbacName)
	if err != nil {
		return nil, err
	}

	if len(hgl) == 0 {
		return nil, fmt.Errorf("header guard not found for name %s: %w", guardRbacName, ErrRecordNotFound)
	}

	id := ScanUUID(hgl[0].PulpHref)
	return ps.RbacGuardRead(ctx, id)
}

// RbacGuardCreate creates a new RBAC guard and returns it.
func (ps *PulpService) RbacGuardCreate(ctx context.Context) (*RBACContentGuardResponse, error) {
	req := RBACContentGuard{
		Name:        guardRbacName,
		Description: ptr.To("EDGE"),
	}
	resp, err := ps.cwr.ContentguardsCoreRbacCreateWithResponse(ctx, ps.dom, req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON201, nil
}

// RbacGuardEnsure ensures that the RBAC guard is created and returns it. The method is idempotent.
func (ps *PulpService) RbacGuardEnsure(ctx context.Context) (*RBACContentGuardResponse, error) {
	cg, err := ps.RbacGuardFind(ctx)
	// nolint: gocritic
	if errors.Is(err, ErrRecordNotFound) {
		cg, err = ps.RbacGuardCreate(ctx)
		if err != nil {
			return nil, err
		}

		return cg, nil
	} else if err != nil {
		return nil, err
	} else if cg == nil {
		return nil, fmt.Errorf("unexpected nil guard")
	}

	u := ptr.FromOrEmpty(cg.Users)
	if !slices.ContainsFunc(u, func(gur GroupUserResponse) bool {
		return gur.Username == rbacUsername()
	}) {
		logrus.WithContext(ctx).Warnf("user %s not found in RBAC Content Guard: %+v, adding it", rbacUsername(), u)
		users := []string{rbacUsername()}
		rreq := NestedRole{
			Role:  rbacUsername(),
			Users: &users,
		}
		id := ScanUUID(cg.PulpHref)
		rresp, err := ps.cwr.ContentguardsCoreRbacAddRoleWithResponse(ctx, ps.dom, id, rreq, addAuthenticationHeader)

		if err != nil {
			return nil, err
		}

		if rresp.JSON201 == nil {
			return nil, fmt.Errorf("unexpected response: %d, body: %s", rresp.StatusCode(), string(rresp.Body))
		}
	}
	return cg, nil
}

// RbacGuardDelete deletes the RBAC guard with the given ID.
func (ps *PulpService) RbacGuardDelete(ctx context.Context, id uuid.UUID) error {
	listParams := ContentguardsCoreRbacListRolesParams{}
	roles, err := ps.cwr.ContentguardsCoreRbacListRolesWithResponse(ctx, ps.dom, id, &listParams, addAuthenticationHeader)

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

		logrus.WithContext(ctx).Warnf("removing RBAC guardrole: %s", role.Role)
		removed, err := ps.cwr.ContentguardsCoreRbacRemoveRoleWithResponse(ctx, ps.dom, id, nr, addAuthenticationHeader)

		if err != nil {
			return err
		}

		if removed.JSON201 == nil {
			return fmt.Errorf("unexpected response: %d, body: %s", removed.StatusCode(), string(removed.Body))
		}
	}

	resp, err := ps.cwr.ContentguardsCoreRbacDelete(ctx, ps.dom, id, addAuthenticationHeader)

	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}

	return nil
}
