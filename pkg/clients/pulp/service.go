package pulp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

// DefaultPageSize is the default page size for listing resources. If a request returns more than
// this limit, an error is returned and proper pagination must be implemented. Chances are, we might not
// need to do this at all.
var DefaultPageSize = 1000

var ErrRecordNotFound = errors.New("record not found")

// nolint: revive
type PulpService struct {
	cwr *ClientWithResponses
	dom string
}

// NewPulpService creates a new PulpService. It uses the org_id from the context to contruct
// domain. If org_id is not found in the context, it returns an error.
func NewPulpService(ctx context.Context) (*PulpService, error) {
	dom, err := common.GetOrgIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting orgID from the context: %w", err)
	}

	return NewPulpServiceWithDomain(ctx, "EDGE-"+dom)
}

var ErrDomainEmpty = errors.New("domain cannot be empty")

// NewPulpServiceWithDomain creates a new PulpService with the given domain. It returns an error
// if the domain is empty.
func NewPulpServiceWithDomain(ctx context.Context, domain string) (*PulpService, error) {
	newClient, err := NewClientWithResponses(config.Get().PulpURL, func(c *Client) error {
		c.Client = clients.NewPlatformClient(ctx, config.Get().PulpProxyURL)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if domain == "" {
		return nil, ErrDomainEmpty
	}

	return &PulpService{cwr: newClient, dom: domain}, nil
}

// NewPulpServiceDefaultDomain creates a new PulpService with the default domain. Use this client
// only for domain creation. All subsequent operations should use the domain specific client.
func NewPulpServiceDefaultDomain(ctx context.Context) (*PulpService, error) {
	return NewPulpServiceWithDomain(ctx, "default")
}

func (ps *PulpService) Domain() string {
	return ps.dom
}

// BackoffDelay allows sleeping for a certain amount of time. The delay pattern is:
// 16ms 32ms 64ms 128ms 256ms 512ms 1.024s 2.048s 2.048s (repeats "forever").
type BackoffDelay int

func (retries *BackoffDelay) Sleep() {
	duration := 2048 * time.Millisecond
	*retries++

	if *retries < 8 {
		duration = (8 << uint(*retries)) * time.Millisecond
	}
	time.Sleep(duration)
}

// WaitForTask waits for the task to complete. It returns the created resource href if the task
// is successful. If the task fails, it returns an error. If the task is cancelled, it returns an
// error.
func (ps *PulpService) WaitForTask(ctx context.Context, taskHref string) ([]string, error) {
	var delay BackoffDelay
	result := make([]string, 0)
	for {
		delay.Sleep()

		taskID := ScanUUID(&taskHref)
		trp := TasksReadParams{}
		task, err := ps.cwr.TasksReadWithResponse(ctx, ps.dom, taskID, &trp, addAuthenticationHeader)
		if err != nil {
			return result, err
		}
		if task.JSON200 == nil || task.JSON200.State == nil {
			return result, fmt.Errorf("unexpected response: %d, body: %s", task.StatusCode(), string(task.Body))
		}

		switch *task.JSON200.State {
		case "completed":
			if task.JSON200.CreatedResources != nil {
				result = append(result, *task.JSON200.CreatedResources...)
			}
			return result, nil
		case "failed":
			return result, fmt.Errorf("task failed (%s): %+v", *task.JSON200.State, *task.JSON200.Error)
		case "cancelled":
			return result, fmt.Errorf("task cancelled: %+v", *task.JSON200)
		}

		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}
}
