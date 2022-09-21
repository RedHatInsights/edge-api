// FIXME: golangci-lint
// nolint:govet,revive
package common

import (
	"context"
	"net/http"
	"testing"
)

func TestGetPagination(t *testing.T) {

	tt := []struct {
		name     string
		expected Pagination
		passed   *Pagination
	}{
		{
			name:     "No pagination set in context should use default pagination",
			expected: Pagination{Offset: defaultOffset, Limit: defaultLimit},
			passed:   nil,
		},
		{
			name:     "Passing pagination to context",
			expected: Pagination{Offset: 10, Limit: 10},
			passed:   &Pagination{Offset: 10, Limit: 10},
		},
	}

	for _, te := range tt {
		req, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Errorf("Test %q: failed create test request: %s", te.name, err)
		}
		if te.passed != nil {
			ctx := context.WithValue(req.Context(), PaginationKey, *te.passed)
			req = req.WithContext(ctx)
		}
		res := GetPagination(req)
		if res.Offset != te.expected.Offset {
			t.Errorf("Test %q: expected pagination offset to be %d but got %d", te.name, te.expected.Offset, res.Offset)
		}
		if res.Limit != te.expected.Limit {
			t.Errorf("Test %q: expected pagination offset to be %d but got %d", te.name, te.expected.Limit, res.Limit)
		}
	}

}
