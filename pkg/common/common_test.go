package common

import (
	"archive/tar"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestStatusOK(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(StatusOK)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := 200
	if rr.Code != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

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

func TestUntar(t *testing.T) {
	//create tar file to be used as mock
	tarPath := "mockTarFile.tar"
	files := map[string]string{
		"index.html":   `<body>Ansible!</body>`,
		"lang.json":    `[{"code":"pt","name":"{Portuguese}"}]`,
		"mock_txt.txt": `some content about red hat`,
	}
	tarWrite := func(data map[string]string) error {
		tarFile, err := os.Create(tarPath)
		if err != nil {
			log.Fatal(err)
		}
		defer tarFile.Close()
		tw := tar.NewWriter(tarFile)
		defer tw.Close()
		for name, content := range data {
			hdr := &tar.Header{
				Name: name,
				Mode: 0600,
				Size: int64(len(content)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := tarWrite(files); err != nil {
		log.Fatal(err)
	}
	unTarFile, errOpenFile := os.Open(tarPath)
	if errOpenFile != nil {
		t.Error("Unable to open mock tar file before test")
	}
	Untar(unTarFile, `./`)
	for name := range files {
		// check if file exist after untar method calls
		if _, err := os.Stat(name); os.IsNotExist(err) {
			t.Fail()
		}
		os.Remove(name)
	}
	os.Remove(tarPath)
}
