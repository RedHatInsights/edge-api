package pulp

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/ptr"
)

func compositeGuardName(orgID string) string {
	return "EDGE_COMP_ORGID=" + orgID
}

// CompositeGuardCreate creates a new composite guard with the given orgID and one or more Pulp guard hrefs.
func (ps *PulpService) CompositeGuardCreate(ctx context.Context, orgID string, hrefs ...string) (*CompositeContentGuardResponse, error) {
	req := ContentguardsCoreCompositeCreateJSONRequestBody{
		Name:        compositeGuardName(orgID),
		Guards:      ptr.To(hrefs),
		Description: ptr.To("EDGE"),
	}
	resp, err := ps.cwr.ContentguardsCoreCompositeCreateWithResponse(ctx, ps.dom, req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON201, nil
}

// CompositeGuardRead returns the composite guard with the given ID.
func (ps *PulpService) CompositeGuardRead(ctx context.Context, id uuid.UUID) (*CompositeContentGuardResponse, error) {
	req := ContentguardsCoreCompositeReadParams{}
	resp, err := ps.cwr.ContentguardsCoreCompositeReadWithResponse(ctx, ps.dom, id, &req, addAuthenticationHeader)

	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	return resp.JSON200, nil
}

// CompositeGuardReadByOrgID returns the composite guard for the given orgID.
func (ps *PulpService) CompositeGuardReadByOrgID(ctx context.Context, orgID string) (*CompositeContentGuardResponse, error) {
	hgl, err := ps.CompositeGuardList(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if len(hgl) == 0 {
		return nil, fmt.Errorf("composite guard not found for orgID %s: %w", orgID, ErrRecordNotFound)
	}

	id := ScanUUID(hgl[0].PulpHref)
	return ps.CompositeGuardRead(ctx, id)
}

func contentGuardHrefsAreEqual(cghrefa, cghrefb []string) bool {
	// compare how many hrefs are in the content guard versus expected
	logrus.Debug("Comparing Content Guard hrefs")
	if len(cghrefa) != len(cghrefb) {
		logrus.WithFields(logrus.Fields{
			"href_count_1": len(cghrefa),
			"href_count_2": len(cghrefb),
		}).Warning("Content Guards do not have the same number of hrefs")

		return false
	}

	logrus.WithFields(logrus.Fields{
		"href_count_1": len(cghrefa),
		"href_count_2": len(cghrefb),
	}).Debug("Content Guards have the same number of hrefs")

	// compare content guard hrefs (actual vs. expected)
	for i := range cghrefa {
		if !slices.Contains(cghrefb, cghrefa[i]) {
			logrus.WithField("href_mismatch", cghrefa[i]).Warning("Content Guard href mismatch")

			return false
		}
	}

	logrus.Debug("Content Guard has the expected hrefs")

	return true
}

// CompositeGuardEnsure ensures that the composite guard is created and returns it. The method is idempotent.
func (ps *PulpService) CompositeGuardEnsure(ctx context.Context, orgID string, pulpGuardHrefs []string) (*CompositeContentGuardResponse, error) {
	cg, err := ps.CompositeGuardReadByOrgID(ctx, compositeGuardName(orgID))
	// nolint: gocritic
	if errors.Is(err, ErrRecordNotFound) {
		logrus.Warning("No composite guard found. Creating one.")
		cg, err = ps.CompositeGuardCreate(ctx, orgID, pulpGuardHrefs...)
		if err != nil {
			return nil, err
		}

		return cg, nil
	} else if err != nil {
		return nil, err
	} else if cg == nil {
		return nil, fmt.Errorf("unexpected nil guard")
	}

	gs := ptr.FromOrEmpty(cg.Guards)
	if !contentGuardHrefsAreEqual(gs, pulpGuardHrefs) {
		logrus.WithContext(ctx).Warnf("unexpected Composite Content Guard: %v, deleting it", gs)
		err = ps.CompositeGuardDelete(ctx, ScanUUID(cg.PulpHref))
		if err != nil {
			return nil, err
		}

		logrus.Warning("Matching composite guard not found. Creating one.")
		cg, err = ps.CompositeGuardCreate(ctx, orgID, pulpGuardHrefs...)
		if err != nil {
			return nil, err
		}
	}
	return cg, nil
}

// ContentGuardEnsure ensures that the header and RBAC guards are created and returns the composite guard. If it finds
// that the composite guard is not created or the guards are not the same as the ones provided, it will delete it
// and recreate it. This method is idempotent and will not create the guards if they already exist.
func (ps *PulpService) ContentGuardEnsure(ctx context.Context, orgID string) (*CompositeContentGuardResponse, error) {
	var contentGuardHrefs []string
	cfg := config.Get()

	logrus.WithContext(ctx).Debug("Creating header content guard")
	hcg, err := ps.HeaderGuardEnsure(ctx, orgID)
	if err != nil {
		return nil, err
	}
	contentGuardHrefs = append(contentGuardHrefs, *hcg.PulpHref)

	// Turnpike Content Guard is primarily for Image Builder to Pulp calls using the URL configured in PULP_CONTENT_URL
	logrus.WithContext(ctx).Debug("Creating turnpike content guard")
	tpcg, err := ps.TurnpikeGuardEnsure(ctx)
	if err != nil {
		return nil, err
	}
	contentGuardHrefs = append(contentGuardHrefs, *tpcg.PulpHref)

	// to create an RBAC Content Guard, add PULP_CONTENT_USERNAME and PULP_CONTENT_PASSWORD to environment
	if cfg.Pulp.ContentUsername != "" {
		logrus.WithContext(ctx).Debug("Creating RBAC content guard")
		rcg, err := ps.RbacGuardEnsure(ctx)
		if err != nil {
			return nil, err
		}
		contentGuardHrefs = append(contentGuardHrefs, *rcg.PulpHref)
	}

	ccg, err := ps.CompositeGuardEnsure(ctx, orgID, contentGuardHrefs)
	if err != nil {
		return nil, err
	}

	return ccg, nil
}

// CompositeGuardDelete deletes the composite guard with the given ID.
func (ps *PulpService) CompositeGuardDelete(ctx context.Context, id uuid.UUID) error {
	resp, err := ps.cwr.ContentguardsCoreCompositeDelete(ctx, ps.dom, id, addAuthenticationHeader)

	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected response: %d", resp.StatusCode)
	}

	return nil
}

// CompositeGuardList returns a list of composite guards with the given name filter.
func (ps *PulpService) CompositeGuardList(ctx context.Context, nameFilter string) ([]CompositeContentGuardResponse, error) {
	req := ContentguardsCoreCompositeListParams{
		Limit: &DefaultPageSize,
	}
	if nameFilter != "" {
		req.Name = &nameFilter
	}

	resp, err := ps.cwr.ContentguardsCoreCompositeListWithResponse(ctx, ps.dom, &req, addAuthenticationHeader)

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
