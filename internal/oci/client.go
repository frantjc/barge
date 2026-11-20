package oci

import (
	"context"
	"net/url"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// newRegistryClient returns a client for the registry that u points to,
// authenticated with the credentials that credential.ForURL finds for it.
// Failing that, the client uses Helm's own credential lookup, which can't
// find anything that credential.Default didn't, but keeps `helm registry
// login` working the way Helm users expect.
func newRegistryClient(ctx context.Context, u *url.URL, location credential.Location) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptWriter(util.StdoutFrom(ctx)),
		registry.ClientOptCredentialsFile(barge.HelmSettings().RegistryConfig),
	}

	if cred, ok := credential.ForURL(ctx, u, location, u.Scheme, "oci"); ok {
		opts = append(opts, registry.ClientOptAuthorizer(auth.Client{
			Credential: auth.StaticCredential(u.Host, auth.Credential{
				Username:     cred.Username,
				Password:     cred.Password,
				AccessToken:  cred.Token,
				RefreshToken: cred.RefreshToken,
			}),
		}))
	}

	if strings.HasSuffix(u.Scheme, "+http") {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	return registry.NewClient(opts...)
}
