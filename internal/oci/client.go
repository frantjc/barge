package oci

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// withRegistryClient calls fn with a client for the registry that u points
// to, authenticated with each of the credentials that credential.Try finds for
// it in turn, until the registry doesn't reject them.
func withRegistryClient(ctx context.Context, u *url.URL, location credential.Location, fn func(*registry.Client) error) error {
	return credential.Try(ctx, u, location, []string{u.Scheme, "oci"}, func(cred *credential.Credential) error {
		r, err := newRegistryClient(ctx, u, cred)
		if err != nil {
			return err
		}

		err = fn(r)
		if isUnauthorized(err) {
			return credential.Rejected(err)
		}

		return err
	})
}

// isUnauthorized reports whether err is the registry rejecting credentials.
func isUnauthorized(err error) bool {
	var errResp *errcode.ErrorResponse
	return errors.As(err, &errResp) && errResp.StatusCode == http.StatusUnauthorized
}

// newRegistryClient returns a client for the registry that u points to,
// authenticated with cred. Without cred, the client uses Helm's own credential
// lookup, which can't find anything that credential.Default didn't, but keeps
// `helm registry login` working the way Helm users expect.
func newRegistryClient(ctx context.Context, u *url.URL, cred *credential.Credential) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptWriter(util.StdoutFrom(ctx)),
		registry.ClientOptCredentialsFile(barge.HelmSettings().RegistryConfig),
	}

	if cred != nil {
		// Unlike Helm's own client, which isn't used here as it's the
		// credentials' authorizer that's replaced, retry transient failures.
		opts = append(opts, registry.ClientOptAuthorizer(auth.Client{
			Client: util.HTTPClient,
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
