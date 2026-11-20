package credential

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// record calls try with a fn that records each credentials that it's called
// with and responds to them with respond, and returns them.
func record(try func(fn func(*Credential) error) error, respond func(*Credential) error) ([]*Credential, error) {
	var tried []*Credential
	err := try(func(cred *Credential) error {
		tried = append(tried, cred)
		return respond(cred)
	})
	return tried, err
}

func acceptAll(*Credential) error { return nil }

func rejectAll(*Credential) error { return Rejected(errors.New("401 Unauthorized")) }

// acceptOnly accepts want and rejects every other credentials.
func acceptOnly(want Credential) func(*Credential) error {
	return func(cred *Credential) error {
		if cred != nil && *cred == want {
			return nil
		}
		return rejectAll(cred)
	}
}

// static is a Provider with fixed credentials.
type static struct {
	name string
	cred *Credential
}

func (s static) String() string { return s.name }

func (s static) Credential(context.Context, *Request) (*Credential, error) {
	return s.cred, nil
}

var (
	wrong = Credential{Username: "u", Password: "wrong"}
	right = Credential{Username: "u", Password: "right"}
)

func TestChainTryFallsThroughRejections(t *testing.T) {
	t.Parallel()
	chain := Chain{static{"a", &wrong}, static{"b", &right}}

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, acceptOnly(right))
	require.NoError(t, err)
	require.Equal(t, []*Credential{&wrong, &right}, tried)
}

func TestChainTryStopsAtSuccess(t *testing.T) {
	t.Parallel()
	asked := false
	chain := Chain{static{"a", &right}, providerFunc(func() { asked = true })}

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, acceptAll)
	require.NoError(t, err)
	require.Equal(t, []*Credential{&right}, tried)
	// Later Providers aren't even asked.
	require.False(t, asked)
}

func TestChainTryStopsAtOtherErrors(t *testing.T) {
	t.Parallel()
	chain := Chain{static{"a", &wrong}, static{"b", &right}}
	notFound := errors.New("404 Not Found")

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, func(*Credential) error { return notFound })
	require.ErrorIs(t, err, notFound)
	require.Equal(t, []*Credential{&wrong}, tried)
}

func TestChainTryOnlyRecognizesRejectedItself(t *testing.T) {
	t.Parallel()
	chain := Chain{static{"a", &wrong}, static{"b", &right}}

	// A rejection wrapped in another error, e.g. from fetching a chart on
	// another host, isn't a rejection of these credentials.
	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, func(cred *Credential) error { return fmt.Errorf("chart: %w", rejectAll(cred)) })
	requireRejected(t, err)
	require.Equal(t, []*Credential{&wrong}, tried)
}

func TestChainTrySkipsDuplicates(t *testing.T) {
	t.Parallel()
	dup := wrong
	chain := Chain{static{"a", &wrong}, static{"b", &dup}, static{"c", &right}}

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, acceptOnly(right))
	require.NoError(t, err)
	require.Equal(t, []*Credential{&wrong, &right}, tried)
}

func TestChainTryReturnsLastRejection(t *testing.T) {
	t.Parallel()
	chain := Chain{static{"a", &wrong}, static{"b", &right}}

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, rejectAll)
	requireRejected(t, err)
	require.Len(t, tried, 2)
}

func TestChainTryAnonymousWithoutCredentials(t *testing.T) {
	t.Parallel()
	chain := Chain{static{"a", nil}, erroringProvider{}}

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, acceptAll)
	require.NoError(t, err)
	require.Equal(t, []*Credential{nil}, tried)
}

func TestChainTryEnvIsFinal(t *testing.T) {
	t.Setenv("BARGE_HTTPS_SOURCE_USERNAME", wrong.Username)
	t.Setenv("BARGE_HTTPS_SOURCE_PASSWORD", wrong.Password)
	chain := Chain{Env{}, static{"b", &right}}

	tried, err := record(func(fn func(*Credential) error) error {
		return chain.Try(t.Context(), request(t, "https://charts.example.com", Source), fn)
	}, acceptOnly(right))
	requireRejected(t, err)
	require.Equal(t, []*Credential{&wrong}, tried)
}

func TestRejected(t *testing.T) {
	t.Parallel()
	cause := errors.New("401 Unauthorized")
	err := Rejected(cause)
	requireRejected(t, err)
	require.ErrorIs(t, err, cause)
	require.Equal(t, "credentials rejected: 401 Unauthorized", err.Error())
}

// requireRejected requires that err is, or wraps, a rejection from Rejected.
func requireRejected(t *testing.T, err error) {
	t.Helper()
	var rejected *rejectedError
	require.ErrorAs(t, err, &rejected)
}

// providerFunc is a Provider without credentials that calls f when asked.
type providerFunc func()

func (f providerFunc) Credential(context.Context, *Request) (*Credential, error) {
	f()
	return nil, nil
}
