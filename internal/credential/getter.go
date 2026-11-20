package credential

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"helm.sh/helm/v3/pkg/getter"
)

// Get gets rawURL with g, authenticating with cred if it's non-nil. Helm's
// getters only support basic authentication, so if cred has a bearer token,
// Get makes a plain HTTP request instead, which only works for HTTP(S) URLs.
func Get(ctx context.Context, g getter.Getter, rawURL string, cred *Credential, opts ...getter.Option) (*bytes.Buffer, error) {
	if cred == nil {
		return g.Get(rawURL, opts...)
	}

	if cred.Token == "" {
		// Helm's HTTP getter only sends basic auth to the scheme and host of
		// the URL given by WithURL.
		return g.Get(rawURL, append(opts, getter.WithURL(rawURL), getter.WithBasicAuth(cred.Username, cred.Password))...)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("bearer tokens are only supported for http(s) URLs, not %s", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	cred.SetAuth(req)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch %s: %s", rawURL, res.Status)
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, res.Body); err != nil {
		return nil, err
	}

	return buf, nil
}
