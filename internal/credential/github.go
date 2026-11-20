package credential

import (
	"context"
	"slices"

	ghauth "github.com/cli/go-gh/v2/pkg/auth"
)

// GitHub finds a token for GitHub and GitHub Enterprise Server hosts the way
// the gh CLI does: $GH_TOKEN/$GITHUB_TOKEN, $GH_ENTERPRISE_TOKEN, or
// `gh auth login`.
//
// It answers for ghcr.io with github.com's token. Otherwise, it only answers
// for hosts that gh knows about, because gh hands $GH_ENTERPRISE_TOKEN to any
// host that isn't github.com.
type GitHub struct{}

func (GitHub) String() string {
	return "github"
}

func (GitHub) Credential(_ context.Context, req *Request) (*Credential, error) {
	host := ghauth.NormalizeHostname(req.URL.Hostname())
	if host == "ghcr.io" {
		host = "github.com"
	}

	if !slices.Contains(ghauth.KnownHosts(), host) {
		return nil, nil
	}

	if token, _ := ghauth.TokenForHost(host); token != "" {
		return &Credential{Username: "x-access-token", Password: token}, nil
	}

	return nil, nil
}
