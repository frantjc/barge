package barge_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/testdata"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"sigs.k8s.io/yaml"
)

const credentialsIsolatedEnv = "BARGE_TEST_CREDENTIALS_ISOLATED"

// isolatedDockerConfig has an auth entry for a host that nothing uses, so
// that credential stores treat it as configured and don't fall back to the
// platform's default credential helper, i.e. the developer's keychain.
const isolatedDockerConfig = `{"auths":{"isolated.invalid":{}}}`

// IsolateCredentials points every place that barge finds credentials in at
// empty temporary locations, so that tests neither use nor modify the
// developer's own credentials and configuration. It's safe to call more than
// once per test.
func IsolateCredentials(t testing.TB) {
	t.Helper()

	if os.Getenv(credentialsIsolatedEnv) != "" {
		return
	}

	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "BARGE_") {
			t.Setenv(k, "")
		}
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NETRC", filepath.Join(home, ".netrc"))

	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST"} {
		t.Setenv(k, "")
	}
	t.Setenv("GH_CONFIG_DIR", filepath.Join(home, ".config", "gh"))

	dockerConfig := filepath.Join(home, ".docker")
	require.NoError(t, os.MkdirAll(dockerConfig, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dockerConfig, "config.json"), []byte(isolatedDockerConfig), 0o600))
	t.Setenv("DOCKER_CONFIG", dockerConfig)

	// The helm CLI reads these from the environment, but barge's settings were
	// read from it at init, so both are set.
	var (
		helmConfig       = filepath.Join(home, ".config", "helm")
		repositoryConfig = filepath.Join(helmConfig, "repositories.yaml")
		repositoryCache  = filepath.Join(home, ".cache", "helm", "repository")
		registryConfig   = filepath.Join(helmConfig, "registry", "config.json")
	)
	require.NoError(t, os.MkdirAll(filepath.Dir(registryConfig), 0o700))
	require.NoError(t, os.WriteFile(registryConfig, []byte(isolatedDockerConfig), 0o600))
	t.Setenv("HELM_CONFIG_HOME", helmConfig)
	t.Setenv("HELM_CACHE_HOME", filepath.Join(home, ".cache", "helm"))
	t.Setenv("HELM_DATA_HOME", filepath.Join(home, ".local", "share", "helm"))
	t.Setenv("HELM_REPOSITORY_CONFIG", repositoryConfig)
	t.Setenv("HELM_REPOSITORY_CACHE", repositoryCache)
	t.Setenv("HELM_REGISTRY_CONFIG", registryConfig)

	settings := barge.HelmSettings()
	prev := *settings
	settings.RepositoryConfig = repositoryConfig
	settings.RepositoryCache = repositoryCache
	settings.RegistryConfig = registryConfig
	t.Cleanup(func() {
		settings.RepositoryConfig = prev.RepositoryConfig
		settings.RepositoryCache = prev.RepositoryCache
		settings.RegistryConfig = prev.RegistryConfig
	})

	t.Setenv(credentialsIsolatedEnv, "1")
}

// Auth is what an AuthServer requires. If Token is set, it requires it as a
// bearer token. Otherwise, it requires Username and Password as basic auth.
type Auth struct {
	Username, Password, Token string
}

// AuthServer is an HTTP server that records the Authorization header of every
// request that it receives and, if it has an Auth, rejects requests that don't
// satisfy it.
type AuthServer struct {
	*httptest.Server
	mu             sync.Mutex
	authorizations []string
}

// NewAuthServer starts an AuthServer in front of handler. If auth is nil,
// every request is let through, which is useful to observe whether
// credentials are sent somewhere that they shouldn't be.
func NewAuthServer(t testing.TB, handler http.Handler, auth *Auth) *AuthServer {
	t.Helper()

	s := &AuthServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		s.mu.Lock()
		s.authorizations = append(s.authorizations, authorization)
		s.mu.Unlock()

		if auth != nil {
			if auth.Token != "" {
				if authorization != "Bearer "+auth.Token {
					// A registry's bearer challenge names a token endpoint, but
					// clients that already have a token use it directly.
					w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="barge"`, s.URL))
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			} else if username, password, ok := r.BasicAuth(); !ok || username != auth.Username || password != auth.Password {
				w.Header().Set("WWW-Authenticate", `Basic realm="barge"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)

	return s
}

// Authorizations returns the Authorization header of every request that s has
// received, "" for requests without one.
func (s *AuthServer) Authorizations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.authorizations...)
}

// RequireAuthorized requires that s received at least one request that
// carried an Authorization header with the given scheme, e.g. "Basic".
func (s *AuthServer) RequireAuthorized(t testing.TB, scheme string) {
	t.Helper()
	for _, a := range s.Authorizations() {
		if strings.HasPrefix(a, scheme+" ") {
			return
		}
	}
	require.Failf(t, "no authorized request", "expected an Authorization header with scheme %s, got %q", scheme, s.Authorizations())
}

// RequireNoAuthorization requires that s never received an Authorization
// header.
func (s *AuthServer) RequireNoAuthorization(t testing.TB) {
	t.Helper()
	for _, a := range s.Authorizations() {
		require.Empty(t, a, "expected no Authorization header")
	}
}

// OtherHostURL returns s's URL with the hostname "localhost" instead of
// "127.0.0.1", so that it's a different host from every other AuthServer as
// far as credentials are concerned, e.g. to netrc, which ignores ports, and to
// Go's HTTP client, which only drops credentials when redirecting to another
// hostname.
func (s *AuthServer) OtherHostURL() string {
	return strings.Replace(s.URL, "127.0.0.1", "localhost", 1)
}

// URLWith returns s's URL with the given scheme, path and userinfo.
func (s *AuthServer) URLWith(t testing.TB, scheme, path string, user *url.Userinfo) *url.URL {
	t.Helper()
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	u.Scheme = scheme
	u.Path = path
	u.User = user
	return u
}

// ChartRepoHandler serves a Helm chart repository with c in it. Its
// index.yaml points at chartURL, which may be relative to the repository or
// absolute, e.g. on another server.
func ChartRepoHandler(t testing.TB, c *chart.Chart, chartURL string) http.Handler {
	t.Helper()

	indexYAML, err := yaml.Marshal(map[string]any{
		"apiVersion": "v1",
		"entries": map[string]any{
			c.Name(): []map[string]any{
				{
					"name":    c.Name(),
					"version": c.Metadata.Version,
					"urls":    []string{chartURL},
				},
			},
		},
	})
	require.NoError(t, err)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.yaml":
			_, _ = w.Write(indexYAML)
		case strings.HasSuffix(r.URL.Path, ".tgz"):
			_, _ = w.Write(testdata.ChartArchive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// ChartFileName is the name of c's archive.
func ChartFileName(c *chart.Chart) string {
	return fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version)
}

// AuthOCI starts an in-process OCI registry that requires auth.
func AuthOCI(t testing.TB, auth *Auth) *AuthServer {
	t.Helper()
	return NewAuthServer(t, registry.New(), auth)
}
