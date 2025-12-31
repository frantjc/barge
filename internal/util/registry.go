package util

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/cli/cli/v2/pkg/cmd/factory"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	authutils "github.com/fluxcd/pkg/auth/utils"
	xslices "github.com/frantjc/x/slices"
	"helm.sh/helm/v3/pkg/registry"
	orasauth "oras.land/oras-go/v2/registry/remote/auth"
)

func NewRegistryClientFromURL(ctx context.Context, u *url.URL) (*registry.Client, error) {
	opts := []registry.ClientOption{registry.ClientOptWriter(StdoutFrom(ctx))}

	if user := u.User; user != nil {
		if password, ok := user.Password(); ok {
			username := user.Username()
			opts = append(opts, registry.ClientOptBasicAuth(username, password))
		}
	} else if provider := u.Query().Get("provider"); provider != "" {
		opts = append(opts, cliOptForURLAndProvider(u, provider))
	} else if hostname := u.Hostname(); hostname == "ghcr.io" {
		if cfg, err := factory.New("v0.0.0-unknown").Config(); err == nil {
			authCfg := cfg.Authentication()
			if user, err := authCfg.ActiveUser("github.com"); err == nil {
				token, _ := authCfg.ActiveToken("github.com")
				opts = append(opts, registry.ClientOptBasicAuth(user, token))
			}
		}
	} else if xslices.Some([]string{".azurecr.io", ".azurecr.us", ".azurecr.cn"}, func(suffix string, _ int) bool {
		return strings.HasSuffix(hostname, suffix)
	}) {
		opts = append(opts, cliOptForURLAndProvider(u, azure.ProviderName))
	} else if strings.HasSuffix(hostname, ".amazonaws.com") {
		opts = append(opts, cliOptForURLAndProvider(u, aws.ProviderName))
	}

	if insecure, _ := strconv.ParseBool(u.Query().Get("insecure")); insecure {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	return registry.NewClient(opts...)
}

func RefFromURL(u *url.URL) string {
	ref := path.Join(u.Host, u.Path)
	if strings.Contains(u.Path, ":") {
		return ref
	}
	return fmt.Sprintf("%s:latest", ref)
}

func cliOptForURLAndProvider(u *url.URL, provider string) registry.ClientOption {
	authOpts := []auth.Option{}
	if provider == azure.ProviderName {
		authOpts = append(authOpts, auth.WithAllowShellOut())
	}

	return registry.ClientOptAuthorizer(orasauth.Client{
		Credential: func(ctx context.Context, _ string) (orasauth.Credential, error) {
			authenticator, err := authutils.GetArtifactRegistryCredentials(ctx, provider, fmt.Sprintf("oci://%s", RefFromURL(u)), authOpts...)
			if err != nil {
				return orasauth.EmptyCredential, err
			}

			authConfig, err := authenticator.Authorization()
			if err != nil {
				return orasauth.EmptyCredential, err
			}

			return orasauth.Credential{
				Username:     authConfig.Username,
				Password:     authConfig.Password,
				RefreshToken: authConfig.IdentityToken,
				AccessToken:  authConfig.RegistryToken,
			}, nil
		},
	})
}
