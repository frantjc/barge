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

// Get gets rawURL, authenticating with cred if it's non-nil. An HTTP(S) URL is
// gotten by a plain HTTP request, so that bearer tokens can be sent and a 401
// is reported as Rejected, for Try to move on to the next credentials. Any
// other URL is gotten by g, e.g. a Helm getter plugin's, which only supports
// basic authentication.
func Get(ctx context.Context, g getter.Getter, rawURL string, cred *Credential, opts ...getter.Option) (*bytes.Buffer, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		switch {
		case cred == nil:
			return g.Get(rawURL, opts...)
		case cred.Token != "":
			return nil, fmt.Errorf("bearer tokens are only supported for http(s) URLs, not %s", u.Scheme)
		}

		// Helm's getters only send basic auth to the scheme and host of the
		// URL given by WithURL.
		return g.Get(rawURL, append(opts, getter.WithURL(rawURL), getter.WithBasicAuth(cred.Username, cred.Password))...)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	if cred != nil {
		cred.SetAuth(req)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if statusCode := res.StatusCode; statusCode == http.StatusUnauthorized {
		return nil, Rejected(fmt.Errorf("failed to fetch %s: %s", rawURL, res.Status))
	} else if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch %s: %s", rawURL, res.Status)
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, res.Body); err != nil {
		return nil, err
	}

	return buf, nil
}
