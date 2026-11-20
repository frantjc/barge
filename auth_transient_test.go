package barge_test

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// flaky responds 503 to the first n requests that reach it, then defers to
// handler.
func flaky(handler http.Handler, n int32) http.Handler {
	var requests atomic.Int32
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) <= n {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// A transient failure is retried with the same credentials, rather than taken
// for a rejection that moves on to the next provider's.
func TestAuthTransientFailureRetriesSameCredentials(t *testing.T) {
	for _, tc := range retryCases() {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			// Behind the AuthServer, so only authorized requests are failed.
			srv := NewAuthServer(t, flaky(tc.handler(t), 1), basicAuth)
			host := hostOf(t, srv.URL)

			netrcFor(t, host, authPassword)
			dockerConfigFor(t, dockerConfigPath(), host, wrongPassword)

			require.NoError(t, tc.use(t, srv, nil))
			require.Positive(t, countHeader(srv, rightHeader))
			require.Zero(t, countHeader(srv, wrongHeader), "the next provider's credentials were tried after a transient failure")
		})
	}
}
