package credential

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/getter"
)

func TestEnvToken(t *testing.T) {
	t.Setenv("BARGE_OCI_DESTINATION_TOKEN", "oci-token")

	cred, err := Env{}.Credential(t.Context(), request(t, "oci://registry.example.com/charts", Destination, "oci"))
	require.NoError(t, err)
	require.Equal(t, &Credential{Token: "oci-token"}, cred)

	// A password for the same scheme takes precedence.
	t.Setenv("BARGE_OCI_DESTINATION_PASSWORD", "oci-pass")
	cred, err = Env{}.Credential(t.Context(), request(t, "oci://registry.example.com/charts", Destination, "oci"))
	require.NoError(t, err)
	require.Equal(t, &Credential{Password: "oci-pass"}, cred)

	// But an earlier scheme's token beats a later scheme's password.
	t.Setenv("BARGE_OCI_HTTP_DESTINATION_TOKEN", "alias-token")
	cred, err = Env{}.Credential(t.Context(), request(t, "oci+http://registry.example.com/charts", Destination, "oci+http", "oci"))
	require.NoError(t, err)
	require.Equal(t, &Credential{Token: "alias-token"}, cred)
}

func TestSetAuth(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "https://charts.example.com", nil)
	(&Credential{Username: "u", Password: "p"}).SetAuth(req)
	username, password, ok := req.BasicAuth()
	require.True(t, ok)
	require.Equal(t, "u", username)
	require.Equal(t, "p", password)

	// A token takes precedence over a username and password.
	(&Credential{Username: "u", Password: "p", Token: "t"}).SetAuth(req)
	require.Equal(t, "Bearer t", req.Header.Get("Authorization"))
}

func TestGet(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	t.Cleanup(srv.Close)

	g, err := getter.NewHTTPGetter()
	require.NoError(t, err)

	buf, err := Get(t.Context(), g, srv.URL, nil)
	require.NoError(t, err)
	require.Empty(t, buf.String())

	buf, err = Get(t.Context(), g, srv.URL, &Credential{Username: "u", Password: "p"})
	require.NoError(t, err)
	require.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p")), buf.String())

	buf, err = Get(t.Context(), g, srv.URL, &Credential{Token: "t"})
	require.NoError(t, err)
	require.Equal(t, "Bearer t", buf.String())

	_, err = Get(t.Context(), g, "s3://bucket/chart.tgz", &Credential{Token: "t"})
	require.ErrorContains(t, err, "only supported for http(s)")
}

func TestGetErrorStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	_, err := Get(t.Context(), nil, srv.URL, &Credential{Token: "t"})
	require.ErrorContains(t, err, "401")
}

func TestHelmRegistry(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("helm-user:helm-pass"))
	path := writeFile(t, "config.json", `{"auths":{"registry.example.com":{"auth":"`+auth+`"}}}`)
	h := HelmRegistry{Path: func() string { return path }}

	cred, err := h.Credential(t.Context(), request(t, "oci://registry.example.com/charts", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "helm-user", Password: "helm-pass"}, cred)

	cred, err = h.Credential(t.Context(), request(t, "oci://unknown.example.com/charts", Source))
	require.NoError(t, err)
	require.Nil(t, cred)

	// A missing file is empty.
	h = HelmRegistry{Path: func() string { return filepath.Join(t.TempDir(), "missing.json") }}
	cred, err = h.Credential(t.Context(), request(t, "oci://registry.example.com/charts", Source))
	require.NoError(t, err)
	require.Nil(t, cred)
}

func TestDockerIdentityToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	auth := base64.StdEncoding.EncodeToString([]byte("00000000-0000-0000-0000-000000000000:"))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.json"),
		[]byte(`{"auths":{"myacr.azurecr.io":{"auth":"`+auth+`","identitytoken":"refresh"}}}`),
		0o600,
	))

	cred, err := Docker{}.Credential(t.Context(), request(t, "oci://myacr.azurecr.io/charts", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "00000000-0000-0000-0000-000000000000", RefreshToken: "refresh"}, cred)
}

func TestGitHubGHCR(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_HOST", "")
	t.Setenv("GH_TOKEN", "gh-token")
	t.Setenv("GITHUB_TOKEN", "")

	cred, err := GitHub{}.Credential(t.Context(), request(t, "oci://ghcr.io/frantjc/charts", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "x-access-token", Password: "gh-token"}, cred)
}

func TestCloudProvider(t *testing.T) {
	t.Parallel()
	for rawURL, want := range map[string]string{
		"oci://myacr.azurecr.io/charts":                             "azure",
		"oci://myacr.azurecr.cn/charts":                             "azure",
		"oci://123456789012.dkr.ecr.us-east-1.amazonaws.com/charts": "aws",
		"oci://us-docker.pkg.dev/project/charts?provider=gcp":       "gcp",
		"oci://myacr.azurecr.io/charts?provider=generic":            "generic",
		"oci://registry.example.com/charts":                         "",
		"https://charts.example.com/stable":                         "",
	} {
		u, err := url.Parse(rawURL)
		require.NoError(t, err)
		require.Equal(t, want, cloudProvider(u), rawURL)
	}
}

func TestArtifactRepository(t *testing.T) {
	t.Parallel()
	for rawURL, want := range map[string]string{
		"oci://myacr.azurecr.io/charts/mychart":        "myacr.azurecr.io/charts/mychart",
		"oci://myacr.azurecr.io/charts/mychart:1.0.0":  "myacr.azurecr.io/charts/mychart",
		"oci://registry.example.com:5000/charts:1.0.0": "registry.example.com:5000/charts",
	} {
		u, err := url.Parse(rawURL)
		require.NoError(t, err)
		require.Equal(t, want, artifactRepository(u), rawURL)
	}
}
