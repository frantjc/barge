package util_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/frantjc/barge/internal/util"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err)
		res, err := util.HTTPClient.Do(req)
		require.NoError(t, err)
		res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode, "%d", status)
		require.EqualValues(t, 2, requests.Load(), "%d", status)
	}
}

func TestHTTPClientReplaysBodies(t *testing.T) {
	t.Parallel()
	srv, _, bodies := failing(t, 1, http.StatusServiceUnavailable)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL, strings.NewReader("chart"))
	require.NoError(t, err)
	res, err := util.HTTPClient.Do(req)
	require.NoError(t, err)
	res.Body.Close()

	require.Equal(t, []string{"chart", "chart"}, *bodies)
}

func TestHTTPClientDoesNotRetryRejections(t *testing.T) {
	t.Parallel()
	srv, requests, _ := failing(t, 1, http.StatusUnauthorized)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	res, err := util.HTTPClient.Do(req)
	require.NoError(t, err)
	res.Body.Close()

	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.EqualValues(t, 1, requests.Load())
}

// dropping hijacks and closes the connection of the first n requests without
// responding, then responds 200.
func dropping(t *testing.T, n int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		if requests.Add(1) <= n {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func TestHTTPClientRetriesDroppedConnections(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut} {
		srv, requests := dropping(t, 2)

		req, err := http.NewRequestWithContext(t.Context(), method, srv.URL, strings.NewReader("chart"))
		require.NoError(t, err)
		res, err := util.HTTPClient.Do(req)
		require.NoError(t, err, method)
		res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode, method)
		require.EqualValues(t, 3, requests.Load(), method)
	}
}

func TestHTTPClientDoesNotRetryDroppedNonIdempotentRequests(t *testing.T) {
	t.Parallel()
	srv, requests := dropping(t, 1)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, strings.NewReader("chart"))
	require.NoError(t, err)
	res, err := util.HTTPClient.Do(req)
	if res != nil {
		res.Body.Close()
	}
	require.Error(t, err)

	require.True(t, util.IsDroppedConn(err), "expected a dropped connection, got %v", err)
	require.EqualValues(t, 1, requests.Load())
}
