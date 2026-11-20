package credential

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type unnamedProvider struct {
	Path func() string
}

func (unnamedProvider) Credential(context.Context, *Request) (*Credential, error) {
	return nil, nil
}

func TestProviderName(t *testing.T) {
	t.Parallel()
	require.Equal(t, "netrc", providerName(Netrc{}))
	require.Equal(t, "helm repositories", providerName(HelmRepositories{Path: func() string { return "" }}))
	// Without a String method, the type, not a dump of its fields.
	require.Equal(t, "credential.unnamedProvider", providerName(unnamedProvider{Path: func() string { return "" }}))
}
