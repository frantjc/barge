package credential

import (
	"context"
	"net/url"
	"path"
	"strings"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	authutils "github.com/fluxcd/pkg/auth/utils"
)

// Cloud gets short-lived credentials for a cloud provider's container
// registry from the ambient identity, e.g. a workload identity or the
// provider's CLI. The provider is named by the Request URL's "provider" query
// parameter, e.g. "?provider=gcp", or else detected from its host, for Azure
// Container Registry and Amazon Elastic Container Registry.
type Cloud struct{}

func (Cloud) String() string {
	return "cloud"
}

func (Cloud) Credential(ctx context.Context, req *Request) (*Credential, error) {
	provider := cloudProvider(req.URL)
	if provider == "" {
		return nil, nil
	}

	opts := []auth.Option{}
	if provider == azure.ProviderName {
		opts = append(opts, auth.WithAllowShellOut())
	}

	authenticator, err := authutils.GetArtifactRegistryCredentials(ctx, provider, artifactRepository(req.URL), opts...)
	if err != nil {
		return nil, err
	}

	authConfig, err := authenticator.Authorization()
	if err != nil {
		return nil, err
	}

	return &Credential{
		Username:     authConfig.Username,
		Password:     authConfig.Password,
		Token:        authConfig.RegistryToken,
		RefreshToken: authConfig.IdentityToken,
	}, nil
}

func cloudProvider(u *url.URL) string {
	if provider := u.Query().Get("provider"); provider != "" {
		return provider
	}

	hostname := strings.ToLower(u.Hostname())
	for _, suffix := range []string{".azurecr.io", ".azurecr.us", ".azurecr.cn"} {
		if strings.HasSuffix(hostname, suffix) {
			return azure.ProviderName
		}
	}

	if strings.HasSuffix(hostname, ".amazonaws.com") {
		return aws.ProviderName
	}

	return ""
}

// artifactRepository returns u as the repository that fluxcd's providers
// expect, e.g. "myregistry.azurecr.io/charts/mychart".
func artifactRepository(u *url.URL) string {
	p, _, _ := strings.Cut(u.Path, ":")
	return path.Join(u.Host, p)
}
