package credential

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func request(t *testing.T, rawURL string, location Location, schemes ...string) *Request {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	if len(schemes) == 0 {
		schemes = []string{u.Scheme}
	}
	return &Request{URL: u, Location: location, Schemes: schemes}
}

func writeFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestForURLPrefersUserinfoAndStripsIt(t *testing.T) {
	t.Setenv("BARGE_HTTPS_SOURCE_PASSWORD", "from-env")

	u, err := url.Parse("https://user:pass@charts.example.com/stable")
	require.NoError(t, err)

	cred, ok := ForURL(t.Context(), u, Source)
	require.True(t, ok)
	require.Equal(t, &Credential{Username: "user", Password: "pass"}, cred)
	require.Nil(t, u.User)
}

func TestEnv(t *testing.T) {
	t.Setenv("BARGE_ARTIFACTORY_DESTINATION_USERNAME", "canonical-user")
	t.Setenv("BARGE_ARTIFACTORY_DESTINATION_PASSWORD", "canonical-pass")

	// The alias the user wrote has no variables, so the canonical name is used.
	cred, err := Env{}.Credential(t.Context(), request(t, "rt+https://rt.example.com", Destination, "rt+https", "artifactory"))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "canonical-user", Password: "canonical-pass"}, cred)

	// The alias wins when it has variables.
	t.Setenv("BARGE_RT_HTTPS_DESTINATION_PASSWORD", "alias-pass")
	cred, err = Env{}.Credential(t.Context(), request(t, "rt+https://rt.example.com", Destination, "rt+https", "artifactory"))
	require.NoError(t, err)
	require.Equal(t, &Credential{Password: "alias-pass"}, cred)

	// Sources don't see destinations' variables.
	cred, err = Env{}.Credential(t.Context(), request(t, "rt+https://rt.example.com", Source, "rt+https", "artifactory"))
	require.NoError(t, err)
	require.Nil(t, cred)
}

var repositoriesYAML = `
apiVersion: ""
generated: "` + time.Now().Format(time.RFC3339) + `"
repositories:
- name: rt-api
  url: https://rt.example.com/artifactory/api/helm/charts
  username: rt-user
  password: rt-pass
- name: rt-other
  url: https://rt.example.com/artifactory/api/helm/other
  username: rt-user
  password: rt-pass
- name: shared-a
  url: https://shared.example.com/a
  username: a
  password: a-pass
- name: shared-b
  url: https://shared.example.com/b
  username: b
  password: b-pass
- name: public
  url: https://public.example.com/charts
`

func TestHelmRepositories(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "repositories.yaml", repositoriesYAML)
	h := HelmRepositories{Path: func() string { return path }}

	for _, tc := range []struct {
		name string
		url  string
		want *Credential
	}{
		{"exact prefix", "https://rt.example.com/artifactory/api/helm/charts/index.yaml", &Credential{Username: "rt-user", Password: "rt-pass"}},
		{"same host, unambiguous", "artifactory+https://rt.example.com/artifactory/helm-local", &Credential{Username: "rt-user", Password: "rt-pass"}},
		{"explicit default port", "https://rt.example.com:443/artifactory/helm-local", &Credential{Username: "rt-user", Password: "rt-pass"}},
		{"different port", "https://rt.example.com:8443/artifactory/helm-local", nil},
		{"plain http default port", "http://rt.example.com/artifactory/helm-local", nil},
		{"same host, prefix disambiguates", "https://shared.example.com/b/index.yaml", &Credential{Username: "b", Password: "b-pass"}},
		{"same host, ambiguous", "https://shared.example.com/c", nil},
		{"path prefix must end at a segment", "https://shared.example.com/bb", nil},
		{"no credentials", "https://public.example.com/charts", nil},
		{"unknown host", "https://unknown.example.com/charts", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cred, err := h.Credential(t.Context(), request(t, tc.url, Source))
			require.NoError(t, err)
			require.Equal(t, tc.want, cred)
		})
	}
}

func TestHelmRepositoriesMissingFile(t *testing.T) {
	t.Parallel()
	h := HelmRepositories{Path: func() string { return filepath.Join(t.TempDir(), "missing.yaml") }}
	cred, err := h.Credential(t.Context(), request(t, "https://rt.example.com", Source))
	require.NoError(t, err)
	require.Nil(t, cred)
}

func TestNetrc(t *testing.T) {
	t.Parallel()
	path := writeFile(t, ".netrc", `
default login catch-all password catch-all-pass

machine rt.example.com
  login rt-user
  password "with space \"quoted\""

macdef init
machine evil.example.com login evil password evil

machine Other.Example.com login other password other-pass account ignored
`)
	n := Netrc{Path: path}

	cred, err := n.Credential(t.Context(), request(t, "https://rt.example.com:8443/charts", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "rt-user", Password: `with space "quoted"`}, cred)

	cred, err = n.Credential(t.Context(), request(t, "https://other.example.com", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "other", Password: "other-pass"}, cred)

	// Lines in a macro definition aren't entries.
	cred, err = n.Credential(t.Context(), request(t, "https://evil.example.com", Source))
	require.NoError(t, err)
	require.Nil(t, cred)

	// The default entry is ignored.
	cred, err = n.Credential(t.Context(), request(t, "https://unknown.example.com", Source))
	require.NoError(t, err)
	require.Nil(t, cred)
}

func TestGitHub(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_HOST", "")
	t.Setenv("GH_TOKEN", "gh-token")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_ENTERPRISE_TOKEN", "enterprise-token")
	t.Setenv("GITHUB_ENTERPRISE_TOKEN", "")

	cred, err := GitHub{}.Credential(t.Context(), request(t, "https://github.com/frantjc/barge", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "x-access-token", Password: "gh-token"}, cred)

	// gh would hand $GH_ENTERPRISE_TOKEN to any host, so unknown hosts get nothing.
	cred, err = GitHub{}.Credential(t.Context(), request(t, "https://charts.example.com", Source))
	require.NoError(t, err)
	require.Nil(t, cred)

	// ...but a host that gh knows about gets it.
	t.Setenv("GH_HOST", "ghe.example.com")
	cred, err = GitHub{}.Credential(t.Context(), request(t, "https://ghe.example.com/org/repo", Source))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "x-access-token", Password: "enterprise-token"}, cred)
}

func TestDocker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	auth := base64.StdEncoding.EncodeToString([]byte("rt-user:rt-pass"))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.json"),
		[]byte(`{"auths":{"rt.example.com":{"auth":"`+auth+`"}}}`),
		0o600,
	))

	cred, err := Docker{}.Credential(t.Context(), request(t, "artifactory+https://rt.example.com/artifactory/helm-local", Destination))
	require.NoError(t, err)
	require.Equal(t, &Credential{Username: "rt-user", Password: "rt-pass"}, cred)

	cred, err = Docker{}.Credential(t.Context(), request(t, "https://unknown.example.com", Source))
	require.NoError(t, err)
	require.Nil(t, cred)
}

type erroringProvider struct{}

func (erroringProvider) String() string { return "erroring" }

func (erroringProvider) Credential(context.Context, *Request) (*Credential, error) {
	return nil, os.ErrPermission
}

func TestChainSkipsErrors(t *testing.T) {
	t.Setenv("BARGE_HTTPS_SOURCE_PASSWORD", "pass")
	cred := Chain{erroringProvider{}, Env{}}.Credential(t.Context(), request(t, "https://charts.example.com", Source))
	require.Equal(t, &Credential{Password: "pass"}, cred)
}
