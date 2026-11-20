package util_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/frantjc/barge/internal/util"
)

// failing responds with status to the first n requests, then 200, recording
// each request's body.
func failing(t *testing.T, n int32, status int) (*httptest.Server, *atomic.Int32, *[]string) {
	t.Helper()
	var requests atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if requests.Add(1) <= n {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &requests, &bodies
}

func TestHTTPClientRetriesTransientFailures(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusBadGateway} {
		srv, requests, _ := failing(t, 1, status)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := util.HTTPClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()

		if res.StatusCode != http.StatusOK || requests.Load() != 2 {
			t.Fatalf("%d: expected a 200 after 2 requests, got %d after %d", status, res.StatusCode, requests.Load())
		}
	}
}

func TestHTTPClientReplaysBodies(t *testing.T) {
	t.Parallel()
	srv, _, bodies := failing(t, 1, http.StatusServiceUnavailable)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL, strings.NewReader("chart"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := util.HTTPClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if len(*bodies) != 2 || (*bodies)[0] != "chart" || (*bodies)[1] != "chart" {
		t.Fatalf("expected the body to be sent twice, got %q", *bodies)
	}
}

func TestHTTPClientDoesNotRetryRejections(t *testing.T) {
	t.Parallel()
	srv, requests, _ := failing(t, 1, http.StatusUnauthorized)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := util.HTTPClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized || requests.Load() != 1 {
		t.Fatalf("expected a 401 after 1 request, got %d after %d", res.StatusCode, requests.Load())
	}
}
