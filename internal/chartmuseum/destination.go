package chartmuseum

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"chartmuseum",
		"cm",
		"chartmuseum+https",
		"cm+https",
	)
	barge.RegisterDestination(
		&destination{"http"},
		"chartmuseum+http",
		"cm+http",
	)
}

type destination struct {
	Scheme string
}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	schemes := []string{u.Scheme, "chartmuseum"}

	u.Scheme = d.Scheme
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	// TODO(frantjc): Use io.Pipe here.
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("chart", fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version))
	if err != nil {
		return err
	}

	rc, err := util.WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	if _, err := io.Copy(fw, rc); err != nil {
		return err
	}

	if err := mw.Close(); err != nil {
		return err
	}

	return credential.Try(ctx, u, credential.Destination, schemes, func(cred *credential.Credential) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.JoinPath("/api/charts").String(), bytes.NewReader(buf.Bytes()))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", mw.FormDataContentType())

		if cred != nil {
			cred.SetAuth(req)
		}

		res, err := util.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if res.StatusCode == http.StatusUnauthorized {
			return credential.Rejected(fmt.Errorf("http status %s", res.Status))
		}

		if statusCode := res.StatusCode; 200 > statusCode || statusCode >= 400 {
			return fmt.Errorf("http status %s", res.Status)
		}

		return nil
	})
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace, _ string, c *chart.Chart) error {
	if namespace != "" {
		return fmt.Errorf("namespace is not supported for chartmuseum destination")
	}

	return d.Write(ctx, u, c)
}
