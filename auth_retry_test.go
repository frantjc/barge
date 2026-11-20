package barge_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/frantjc/barge"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/require"
)

// wrongPassword is rejected by every AuthServer.
const wrongPassword = "not-the-password"

func basicHeader(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

var (
	rightHeader = basicHeader(authUser, authPassword)
	wrongHeader = basicHeader(authUser, wrongPassword)
)

func hostOf(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u
}

// netrcFor writes a netrc entry for host. netrc comes before Docker in the
// credential chain.
func netrcFor(t *testing.T, host *url.URL, password string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		os.Getenv("NETRC"),
		fmt.Appendf(nil, "machine %s login %s password %s\n", host.Hostname(), authUser, password),
		0o600,
	))
}

// dockerConfigFor writes a Docker-style config with credentials for host to
// path.
func dockerConfigFor(t *testing.T, path string, host *url.URL, password string) {
	t.Helper()
	auth := base64.StdEncoding.EncodeToString([]byte(authUser + ":" + password))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `{"auths":{%q:{"auth":%q}}}`, host.Host, auth), 0o600))
}

func dockerConfigPath() string {
	return filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json")
}

func countHeader(srv *AuthServer, header string) int {
	n := 0
	for _, a := range srv.Authorizations() {
		if a == header {
			n++
		}
	}
	return n
}

// retryCase uses srv, which requires basicAuth, as one protocol.
type retryCase struct {
	name    string
	handler func(*testing.T) http.Handler
	use     func(t *testing.T, srv *AuthServer, user *url.Userinfo) error
}

func retryCases() []retryCase {
	return []retryCase{
		{
			name: "http source",
			handler: func(t *testing.T) http.Handler {
				c := testChart(t)
				return ChartRepoHandler(t, c, ChartFileName(c))
			},
			use: func(t *testing.T, srv *AuthServer, user *url.Userinfo) error {
				c := testChart(t)
				return barge.Copy(Context(t), srv.URLWith(t, "http", "/"+ChartFileName(c), user).String(), t.TempDir())
			},
		},
		{
			name: "artifactory destination",
			handler: func(*testing.T) http.Handler {
				uploaded := false
				return uploadHandler(http.MethodPut, &uploaded)
			},
			use: func(t *testing.T, srv *AuthServer, user *url.Userinfo) error {
				_, archive := Archive(t)
				return barge.Copy(Context(t), archive.String(), srv.URLWith(t, "artifactory+http", "/artifactory/helm-local", user).String())
			},
		},
		{
			name: "chartmuseum destination",
			handler: func(*testing.T) http.Handler {
				uploaded := false
				return uploadHandler(http.MethodPost, &uploaded)
			},
			use: func(t *testing.T, srv *AuthServer, user *url.Userinfo) error {
				_, archive := Archive(t)
				return barge.Copy(Context(t), archive.String(), srv.URLWith(t, "cm+http", "", user).String())
			},
		},
		{
			name: "oci",
			handler: func(*testing.T) http.Handler {
				return registry.New()
			},
			use: func(t *testing.T, srv *AuthServer, user *url.Userinfo) error {
				_, archive := Archive(t)
				u := srv.URLWith(t, "oci+http", "/charts/test", user)
				if err := barge.Copy(Context(t), archive.String(), u.String()); err != nil {
					return err
				}
				return barge.Copy(Context(t), srv.URLWith(t, "oci+http", "/charts/test", user).String(), t.TempDir())
			},
		},
	}
}

// Rejected credentials from one host-scoped provider fall through to the next
// provider's.
func TestAuthRetryFallsThroughToNextProvider(t *testing.T) {
	for _, tc := range retryCases() {
		t.Run(tc.name, func(t *testing.T) {
			IsolateCredentials(t)
			srv := NewAuthServer(t, tc.handler(t), basicAuth)
			host := hostOf(t, srv.URL)

			netrcFor(t, host, wrongPassword)
			dockerConfigFor(t, dockerConfigPath(), host, authPassword)

			require.NoError(t, tc.use(t, srv, nil))
			require.Positive(t, countHeader(srv, wrongHeader), "the first provider's credentials weren't tried")
			require.Positive(t, countHeader(srv, rightHeader), "the next provider's credentials weren't tried")
		})
	}
}

// Credentials that the user gave explicitly, via the environment or the URL,
// are final: when they're rejected, other providers' credentials aren't sent.
func TestAuthRetryExplicitCredentialsAreFinal(t *testing.T) {
	for _, tc := range retryCases() {
		for _, explicit := range []string{"env", "userinfo"} {
			t.Run(tc.name+"/"+explicit, func(t *testing.T) {
				IsolateCredentials(t)
				srv := NewAuthServer(t, tc.handler(t), basicAuth)
				dockerConfigFor(t, dockerConfigPath(), hostOf(t, srv.URL), authPassword)

				var user *url.Userinfo
				if explicit == "userinfo" {
					user = url.UserPassword(authUser, wrongPassword)
				} else {
					for _, prefix := range []string{
						"BARGE_HTTP_SOURCE", "BARGE_ARTIFACTORY_DESTINATION", "BARGE_CHARTMUSEUM_DESTINATION",
						"BARGE_OCI_SOURCE", "BARGE_OCI_DESTINATION",
					} {
						t.Setenv(prefix+"_USERNAME", authUser)
						t.Setenv(prefix+"_PASSWORD", wrongPassword)
					}
				}

				err := tc.use(t, srv, user)
				require.Error(t, err)
				requireNoSecrets(t, err.Error())
				require.Positive(t, countHeader(srv, wrongHeader), "the explicit credentials weren't tried")
				require.Zero(t, countHeader(srv, rightHeader), "another provider's credentials were sent after the explicit ones were rejected")
			})
		}
	}
}

// The same credentials from two providers are only tried once.
func TestAuthRetrySkipsDuplicateCredentials(t *testing.T) {
	IsolateCredentials(t)
	c := testChart(t)
	srv := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), basicAuth)
	host := hostOf(t, srv.URL)

	// helm registry config and netrc both come before Docker.
	dockerConfigFor(t, os.Getenv("HELM_REGISTRY_CONFIG"), host, wrongPassword)
	netrcFor(t, host, wrongPassword)
	dockerConfigFor(t, dockerConfigPath(), host, authPassword)

	require.NoError(t, barge.Copy(Context(t), srv.URLWith(t, "http", "/"+ChartFileName(c), nil).String(), t.TempDir()))
	require.Equal(t, 1, countHeader(srv, wrongHeader))
	require.Equal(t, 1, countHeader(srv, rightHeader))
}

// A chart that an index puts on another host gets that host's credentials,
// falling through rejections there too, and a rejection there doesn't make the
// index's credentials be retried.
func TestAuthRetryReferencedChart(t *testing.T) {
	IsolateCredentials(t)
	c := testChart(t)
	other := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), basicAuth)
	otherHost := hostOf(t, other.OtherHostURL())
	srv := NewAuthServer(t, ChartRepoHandler(t, c, other.OtherHostURL()+"/"+ChartFileName(c)), nil)

	netrcFor(t, otherHost, wrongPassword)
	dockerConfigFor(t, dockerConfigPath(), otherHost, authPassword)

	u := srv.URLWith(t, "http", "/"+c.Name(), nil)
	u.RawQuery = url.Values{"version": {c.Metadata.Version}}.Encode()
	require.NoError(t, barge.Copy(Context(t), u.String(), t.TempDir()))

	require.Equal(t, 1, countHeader(other, wrongHeader))
	require.Equal(t, 1, countHeader(other, rightHeader))
	// The index was fetched once, anonymously: the rejection by the chart's
	// host didn't make barge retry the index with other credentials.
	require.Equal(t, []string{""}, srv.Authorizations())
}
