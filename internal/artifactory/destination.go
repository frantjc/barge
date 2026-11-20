package artifactory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/chart"
)

const (
	Scheme = "artifactory"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		Scheme,
		"rt",
		"jfrog",
		Scheme+"+https",
		"rt+https",
		"jfrog+https",
	)
	barge.RegisterDestination(
		&destination{"http"},
		Scheme+"+http",
		"rt+http",
		"jfrog+http",
	)
}

type destination struct {
	Scheme string
}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	schemes := []string{u.Scheme, Scheme}

	u.Scheme = d.Scheme
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	rc, err := util.WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	// Read into memory to send again with each credentials that are tried.
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	return credential.Try(ctx, u, credential.Destination, schemes, func(cred *credential.Credential) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.JoinPath(fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version)).String(), bytes.NewReader(data))
		if err != nil {
			return err
		}

		if cred != nil {
			cred.SetAuth(req)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if statusCode := res.StatusCode; statusCode == http.StatusUnauthorized {
			return credential.Rejected(fmt.Errorf("http status %s", res.Status))
		} else if 200 > statusCode || statusCode >= 400 {
			return fmt.Errorf("http status %s", res.Status)
		}

		return nil
	})
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace, _ string, c *chart.Chart) error {
	return d.Write(ctx, u.JoinPath(namespace), c)
}
