package common

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	. "github.com/onsi/gomega"
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

func getPath(limit, offset int) string {
	path := "/"
	params := make([]string, 0, 2)
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", offset))
	}

	if len(params) > 0 {
		path = path + "?" + strings.Join(params, " ")
	}

	return path
}

func TestPaginate(t *testing.T) {
	RegisterTestingT(t)
	tt := []struct {
		name     string
		limit    int
		offset   int
		expected Pagination
	}{
		{name: "check limit value 10 offset 20", limit: 10, offset: 20, expected: Pagination{Limit: 10, Offset: 20}},
		{name: "check limit value 10 offset 0", limit: 10, offset: 0, expected: Pagination{Limit: 10, Offset: defaultOffset}},
		{name: "check limit value 0 offset 0", limit: 0, offset: 0, expected: Pagination{Limit: defaultLimit, Offset: defaultOffset}},
		{name: "check limit value 10 offset 20", limit: 0, offset: 20, expected: Pagination{Limit: defaultLimit, Offset: 20}},
	}

	for _, te := range tt {
		t.Run(te.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Use(Paginate)
			router.Get("/", func(w http.ResponseWriter, r *http.Request) {
				pagination, ok := r.Context().Value(PaginationKey).(Pagination)
				Expect(ok).To(BeTrue())
				Expect(pagination.Limit).To(Equal(te.expected.Limit))
				Expect(pagination.Offset).To(Equal(te.expected.Offset))
				w.WriteHeader(http.StatusOK)
			})
			req, err := http.NewRequest("GET", getPath(te.limit, te.offset), nil)
			Expect(err).ToNot(HaveOccurred())
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})
	}
}
