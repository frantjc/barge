package util

import (
	"net/http"

	"oras.land/oras-go/v2/registry/remote/retry"
)

// HTTPClient is the client for barge's own HTTP requests. Like oras's, it
// retries requests that fail transiently, with a 408, 429 or 5xx or a network
// timeout, with exponential backoff, rewinding request bodies that can be. It
// doesn't retry other failures, e.g. a 401, which credential.Try handles by
// moving on to other credentials.
var HTTPClient = &http.Client{
	Transport: retry.NewTransport(http.DefaultTransport),
}
