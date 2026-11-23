package util

import (
	"errors"
	"io"
	"net/http"
	"syscall"

	"oras.land/oras-go/v2/registry/remote/retry"
)

// HTTPClient is the client for barge's own HTTP requests. Like oras's, it
// retries requests that fail transiently, with a 408, 429 or 5xx or a network
// timeout, with exponential backoff, rewinding request bodies that can be. It
// also retries idempotent requests whose connection was dropped, e.g. by a
// server or load balancer shedding load, which surfaces as an EOF or a reset.
// It doesn't retry other failures, e.g. a 401, which credential.Try handles by
// moving on to other credentials.
var HTTPClient = &http.Client{
	Transport: &idempotencyTransport{
		idempotent: &retry.Transport{
			Base:   http.DefaultTransport,
			Policy: func() retry.Policy { return droppedConnPolicy },
		},
		other: retry.NewTransport(http.DefaultTransport),
	},
}

var droppedConnPolicy = &retry.GenericPolicy{
	Retryable: func(resp *http.Response, err error) (bool, error) {
		if err != nil && IsDroppedConn(err) {
			return true, nil
		}
		return retry.DefaultPredicate(resp, err)
	},
	Backoff:  retry.DefaultBackoff,
	MinWait:  retry.DefaultPolicy.(*retry.GenericPolicy).MinWait,
	MaxWait:  retry.DefaultPolicy.(*retry.GenericPolicy).MaxWait,
	MaxRetry: retry.DefaultPolicy.(*retry.GenericPolicy).MaxRetry,
}

// IsDroppedConn reports whether err means the connection was closed before a
// response was received.
func IsDroppedConn(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

// idempotencyTransport sends idempotent requests through a transport that
// also retries dropped connections. A non-idempotent request, e.g. a POST,
// might have been processed before its connection was dropped, so retrying it
// could, e.g., upload a chart twice.
type idempotencyTransport struct {
	idempotent, other http.RoundTripper
}

func (t *idempotencyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if isIdempotent(req) {
		return t.idempotent.RoundTrip(req)
	}
	return t.other.RoundTrip(req)
}

// isIdempotent mirrors net/http's notion of a request that's safe to retry.
func isIdempotent(req *http.Request) bool {
	switch req.Method {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, http.MethodPut, http.MethodDelete:
		return true
	}
	_, hasKey := req.Header["Idempotency-Key"]
	_, hasXKey := req.Header["X-Idempotency-Key"]
	return hasKey || hasXKey
}
