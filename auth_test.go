package barge_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/artifactory"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/git"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"sigs.k8s.io/yaml"
)

const (
	authUser     = "barge"
	authPassword = "s3cr3t-password"
	authToken    = "s3cr3t-token"
)

var (
	basicAuth  = &Auth{Username: authUser, Password: authPassword}
	bearerAuth = &Auth{Token: authToken}
)

// requireNoSecrets requires that s contains none of the test's secrets.
func requireNoSecrets(t testing.TB, s string) {
	t.Helper()
	require.NotContains(t, s, authPassword)
	require.NotContains(t, s, authToken)
}

func writeNetrc(t *testing.T, srv *AuthServer) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		os.Getenv("NETRC"),
		fmt.Appendf(nil, "machine %s login %s password %s\n", u.Hostname(), authUser, authPassword),
		0o600,
	))
}

// writeDockerConfig writes basicAuth for srv's host to a Docker-style config
// file, as `docker login` and `helm registry login` do.
func writeDockerConfig(t *testing.T, path string, srv *AuthServer) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	auth := base64.StdEncoding.EncodeToString([]byte(authUser + ":" + authPassword))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `{"auths":{%q:{"auth":%q}}}`, u.Host, auth), 0o600))
}

func helmRepoAdd(t *testing.T, srv *AuthServer) string {
	t.Helper()
	name := uuid.NewString()
	require.NoError(t, Command(t, "helm", "repo", "add", name, srv.URL, "--username", authUser, "--password", authPassword).Run())
	return name
}

// authCase configures credentials for srv the way a user would, returning the
// URL to give barge.
type authCase struct {
	name      string
	auth      *Auth
	configure func(t *testing.T, srv *AuthServer, u *url.URL) *url.URL
}

func userinfoCase() authCase {
	return authCase{"userinfo", basicAuth, func(_ *testing.T, _ *AuthServer, u *url.URL) *url.URL {
		u.User = url.UserPassword(authUser, authPassword)
		return u
	}}
}

func envBasicCase(prefix string) authCase {
	return authCase{"env basic", basicAuth, func(t *testing.T, _ *AuthServer, u *url.URL) *url.URL {
		t.Setenv(prefix+"_USERNAME", authUser)
		t.Setenv(prefix+"_PASSWORD", authPassword)
		return u
	}}
}

func envTokenCase(prefix string) authCase {
	return authCase{"env token", bearerAuth, func(t *testing.T, _ *AuthServer, u *url.URL) *url.URL {
		t.Setenv(prefix+"_TOKEN", authToken)
		return u
	}}
}

func netrcCase() authCase {
	return authCase{"netrc", basicAuth, func(t *testing.T, srv *AuthServer, u *url.URL) *url.URL {
		writeNetrc(t, srv)
		return u
	}}
}

func authScheme(a *Auth) string {
	if a.Token != "" {
		return "Bearer"
	}
	return "Basic"
}

func testChart(t *testing.T) *chart.Chart {
	t.Helper()
	c, _ := Archive(t)
	return c
}

func TestAuthHTTPSource(t *testing.T) {
	for _, tc := range []authCase{
		userinfoCase(),
		envBasicCase("BARGE_HTTP_SOURCE"),
		envTokenCase("BARGE_HTTP_SOURCE"),
		netrcCase(),
		{"helm repositories", basicAuth, func(t *testing.T, srv *AuthServer, u *url.URL) *url.URL {
			helmRepoAdd(t, srv)
			return u
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			c := testChart(t)
			srv := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), tc.auth)

			for name, u := range map[string]*url.URL{
				"archive": srv.URLWith(t, "http", "/"+ChartFileName(c), nil),
				"index":   srv.URLWith(t, "http", "/"+c.Name(), nil),
			} {
				t.Run(name, func(t *testing.T) {
					u = tc.configure(t, srv, u)
					q := u.Query()
					q.Set("version", c.Metadata.Version)
					u.RawQuery = q.Encode()

					require.NoError(t, barge.Copy(Context(t), u.String(), t.TempDir()))
					srv.RequireAuthorized(t, authScheme(tc.auth))
				})
			}
		})
	}
}

func TestAuthHTTPSourceRejected(t *testing.T) {
	IsolateCredentials(t)
	c := testChart(t)
	srv := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), basicAuth)
	u := srv.URLWith(t, "http", "/"+ChartFileName(c), nil)

	ctx := Context(t)

	err := barge.Copy(ctx, u.String(), t.TempDir())
	require.ErrorContains(t, err, "401")

	t.Setenv("BARGE_HTTP_SOURCE_USERNAME", authUser)
	t.Setenv("BARGE_HTTP_SOURCE_PASSWORD", authPassword+"-wrong")
	err = barge.Copy(ctx, u.String(), t.TempDir())
	require.ErrorContains(t, err, "401")
	requireNoSecrets(t, err.Error())
}

// A repository's credentials must not be sent to charts that its index puts
// on another host, as with Helm without --pass-credentials.
func TestAuthHTTPSourceIndexDoesNotLeakToOtherHost(t *testing.T) {
	for _, tc := range []authCase{
		userinfoCase(),
		envBasicCase("BARGE_HTTP_SOURCE"),
		envTokenCase("BARGE_HTTP_SOURCE"),
		netrcCase(),
	} {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			c := testChart(t)
			other := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), nil)
			srv := NewAuthServer(t, ChartRepoHandler(t, c, other.OtherHostURL()+"/"+ChartFileName(c)), tc.auth)

			u := tc.configure(t, srv, srv.URLWith(t, "http", "/"+c.Name(), nil))
			u.RawQuery = url.Values{"version": {c.Metadata.Version}}.Encode()

			require.NoError(t, barge.Copy(Context(t), u.String(), t.TempDir()))
			srv.RequireAuthorized(t, authScheme(tc.auth))
			other.RequireNoAuthorization(t)
		})
	}
}

// Credentials must not follow a redirect to another host.
func TestAuthHTTPSourceRedirectDoesNotLeakToOtherHost(t *testing.T) {
	for _, tc := range []authCase{
		envBasicCase("BARGE_HTTP_SOURCE"),
		envTokenCase("BARGE_HTTP_SOURCE"),
	} {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			c := testChart(t)
			other := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), nil)
			srv := NewAuthServer(t, http.RedirectHandler(other.OtherHostURL()+"/"+ChartFileName(c), http.StatusFound), tc.auth)

			u := tc.configure(t, srv, srv.URLWith(t, "http", "/"+ChartFileName(c), nil))

			require.NoError(t, barge.Copy(Context(t), u.String(), t.TempDir()))
			srv.RequireAuthorized(t, authScheme(tc.auth))
			other.RequireNoAuthorization(t)
		})
	}
}

func TestAuthRepoSource(t *testing.T) {
	IsolateCredentials(t)
	c := testChart(t)
	srv := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), basicAuth)
	name := helmRepoAdd(t, srv)

	require.NoError(t, barge.Copy(Context(t), fmt.Sprintf("repo://%s/%s?version=%s", name, c.Name(), c.Metadata.Version), t.TempDir()))
	srv.RequireAuthorized(t, "Basic")
}

func TestAuthRepoSourceDoesNotLeakToOtherHost(t *testing.T) {
	IsolateCredentials(t)
	c := testChart(t)
	other := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), nil)
	srv := NewAuthServer(t, ChartRepoHandler(t, c, other.OtherHostURL()+"/"+ChartFileName(c)), basicAuth)
	name := helmRepoAdd(t, srv)

	require.NoError(t, barge.Copy(Context(t), fmt.Sprintf("repo://%s/%s?version=%s", name, c.Name(), c.Metadata.Version), t.TempDir()))
	other.RequireNoAuthorization(t)
}

func TestAuthOCI(t *testing.T) {
	for _, tc := range []authCase{
		userinfoCase(),
		{"env basic", basicAuth, func(t *testing.T, _ *AuthServer, u *url.URL) *url.URL {
			for _, location := range []string{"SOURCE", "DESTINATION"} {
				t.Setenv("BARGE_OCI_"+location+"_USERNAME", authUser)
				t.Setenv("BARGE_OCI_"+location+"_PASSWORD", authPassword)
			}
			return u
		}},
		{"env token", bearerAuth, func(t *testing.T, _ *AuthServer, u *url.URL) *url.URL {
			t.Setenv("BARGE_OCI_SOURCE_TOKEN", authToken)
			t.Setenv("BARGE_OCI_DESTINATION_TOKEN", authToken)
			return u
		}},
		{"docker config", basicAuth, func(t *testing.T, srv *AuthServer, u *url.URL) *url.URL {
			writeDockerConfig(t, filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"), srv)
			return u
		}},
		{"helm registry config", basicAuth, func(t *testing.T, srv *AuthServer, u *url.URL) *url.URL {
			writeDockerConfig(t, barge.HelmSettings().RegistryConfig, srv)
			return u
		}},
		netrcCase(),
	} {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			_, archive := Archive(t)
			srv := AuthOCI(t, tc.auth)

			push := tc.configure(t, srv, srv.URLWith(t, "oci+http", "/charts/test", nil))
			require.NoError(t, barge.Copy(Context(t), archive.String(), push.String()))

			pull := tc.configure(t, srv, srv.URLWith(t, "oci+http", "/charts/test", nil))
			require.NoError(t, barge.Copy(Context(t), pull.String(), t.TempDir()))

			srv.RequireAuthorized(t, authScheme(tc.auth))
		})
	}
}

func TestAuthOCIRejected(t *testing.T) {
	IsolateCredentials(t)
	_, archive := Archive(t)
	srv := AuthOCI(t, basicAuth)
	u := srv.URLWith(t, "oci+http", "/charts/test", nil)

	ctx := Context(t)

	require.Error(t, barge.Copy(ctx, archive.String(), u.String()))

	t.Setenv("BARGE_OCI_DESTINATION_USERNAME", authUser)
	t.Setenv("BARGE_OCI_DESTINATION_PASSWORD", authPassword+"-wrong")
	err := barge.Copy(ctx, archive.String(), u.String())
	require.Error(t, err)
	requireNoSecrets(t, err.Error())
}

// Copying between two registries uses each one's credentials for the
// direction it's used in.
func TestAuthOCISourceAndDestinationCredentials(t *testing.T) {
	IsolateCredentials(t)
	_, archive := Archive(t)
	authA := &Auth{Username: "a", Password: authPassword + "-a"}
	authB := &Auth{Username: "b", Password: authPassword + "-b"}
	a := AuthOCI(t, authA)
	b := AuthOCI(t, authB)
	aURL := a.URLWith(t, "oci+http", "/charts/test", nil).String()
	bURL := b.URLWith(t, "oci+http", "/charts/test", nil).String()

	set := func(location string, auth *Auth) {
		t.Setenv("BARGE_OCI_"+location+"_USERNAME", auth.Username)
		t.Setenv("BARGE_OCI_"+location+"_PASSWORD", auth.Password)
	}

	set("DESTINATION", authA)
	require.NoError(t, barge.Copy(Context(t), archive.String(), aURL))

	set("SOURCE", authA)
	set("DESTINATION", authB)
	require.NoError(t, barge.Copy(Context(t), aURL, bURL))

	set("SOURCE", authB)
	require.NoError(t, barge.Copy(Context(t), bURL, t.TempDir()))
}

func TestAuthOCISync(t *testing.T) {
	IsolateCredentials(t)
	c, archive := Archive(t)
	srv := AuthOCI(t, bearerAuth)
	t.Setenv("BARGE_OCI_SOURCE_TOKEN", authToken)
	t.Setenv("BARGE_OCI_DESTINATION_TOKEN", authToken)

	root := srv.URLWith(t, "oci+http", "/charts", nil)
	require.NoError(t, barge.Copy(Context(t), archive.String(), ociChartURL(root, c).String()))

	constraints, err := semver.NewConstraint(c.Metadata.Version)
	require.NoError(t, err)
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*root),
				Charts: map[string]barge.Constraints{
					c.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(t, err)
	cfg := filepath.Join(t.TempDir(), "sync.yml")
	require.NoError(t, os.WriteFile(cfg, b, 0o600))

	dest := t.TempDir()
	require.NoError(t, barge.Sync(Context(t), cfg, dest))
	entries, err := os.ReadDir(dest)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
}

// ociChartURL is the URL of c's version in the registry at root.
func ociChartURL(root *url.URL, c *chart.Chart) *url.URL {
	u := root.JoinPath(c.Name())
	u.RawQuery = url.Values{"version": {c.Metadata.Version}}.Encode()
	return u
}

// Sync logs its destination, which mustn't include credentials.
func TestAuthSyncDoesNotLogDestinationCredentials(t *testing.T) {
	IsolateCredentials(t)
	c, archive := Archive(t)
	src := OCI(t)
	require.NoError(t, barge.Copy(Context(t), archive.String(), ociChartURL(src, c).String()))
	srv := AuthOCI(t, basicAuth)

	constraints, err := semver.NewConstraint(c.Metadata.Version)
	require.NoError(t, err)
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*src),
				Charts: map[string]barge.Constraints{
					c.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(t, err)
	cfg := filepath.Join(t.TempDir(), "sync.yml")
	require.NoError(t, os.WriteFile(cfg, b, 0o600))

	ctx := Context(t)
	dest := srv.URLWith(t, "oci+http", "/charts", url.UserPassword(authUser, authPassword))
	require.NoError(t, barge.Sync(ctx, cfg, dest.String()))
	srv.RequireAuthorized(t, "Basic")
}

// uploadHandler accepts the given method and records that it was called.
func uploadHandler(method string, uploaded *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		*uploaded = true
		w.WriteHeader(http.StatusCreated)
	})
}

func TestAuthArtifactoryDestination(t *testing.T) {
	for _, scheme := range []string{"artifactory+http", "rt+http", "jfrog+http"} {
		for _, tc := range []authCase{
			userinfoCase(),
			// The canonical name works whichever alias is used.
			envBasicCase("BARGE_ARTIFACTORY_DESTINATION"),
			envTokenCase("BARGE_ARTIFACTORY_DESTINATION"),
			netrcCase(),
		} {
			t.Run(scheme+"/"+tc.name, func(t *testing.T) {
				IsolateCredentials(t)
				_, archive := Archive(t)
				uploaded := false
				srv := NewAuthServer(t, uploadHandler(http.MethodPut, &uploaded), tc.auth)

				u := tc.configure(t, srv, srv.URLWith(t, scheme, "/artifactory/helm-local", nil))
				require.NoError(t, barge.Copy(Context(t), archive.String(), u.String()))
				require.True(t, uploaded)
				srv.RequireAuthorized(t, authScheme(tc.auth))
			})
		}
	}
}

func TestAuthArtifactoryDestinationRejected(t *testing.T) {
	IsolateCredentials(t)
	_, archive := Archive(t)
	uploaded := false
	srv := NewAuthServer(t, uploadHandler(http.MethodPut, &uploaded), basicAuth)
	u := srv.URLWith(t, "artifactory+http", "/artifactory/helm-local", nil)

	ctx := Context(t)

	require.ErrorContains(t, barge.Copy(ctx, archive.String(), u.String()), "401")

	// Source credentials aren't used for a destination.
	t.Setenv("BARGE_ARTIFACTORY_SOURCE_USERNAME", authUser)
	t.Setenv("BARGE_ARTIFACTORY_SOURCE_PASSWORD", authPassword)
	err := barge.Copy(ctx, archive.String(), u.String())
	require.ErrorContains(t, err, "401")
	require.False(t, uploaded)
	requireNoSecrets(t, err.Error())
}

func TestAuthChartmuseumDestination(t *testing.T) {
	for _, tc := range []authCase{
		userinfoCase(),
		envBasicCase("BARGE_CHARTMUSEUM_DESTINATION"),
		envTokenCase("BARGE_CHARTMUSEUM_DESTINATION"),
		netrcCase(),
	} {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			_, archive := Archive(t)
			uploaded := false
			srv := NewAuthServer(t, uploadHandler(http.MethodPost, &uploaded), tc.auth)

			u := tc.configure(t, srv, srv.URLWith(t, "cm+http", "", nil))
			require.NoError(t, barge.Copy(Context(t), archive.String(), u.String()))
			require.True(t, uploaded)
			srv.RequireAuthorized(t, authScheme(tc.auth))
		})
	}
}
