package artifactory

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/frantjc/barge"
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
	scheme := u.Scheme
	if d.Scheme == "" {
		d.Scheme = "https"
	}
	u.Scheme = d.Scheme

	rc, err := util.WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.JoinPath(fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version)).String(), rc)
	if err != nil {
		return err
	}

	if username, password, ok := util.UsernameAndPasswordForURLWithEnvFallback(u, util.LocationSource, scheme); ok {
		req.Header.Add(
			"Authorization",
			fmt.Sprintf(
				"Basic %s",
				base64.RawURLEncoding.EncodeToString(
					[]byte(fmt.Sprintf("%s:%s", username, password)),
				),
			),
		)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if statusCode := res.StatusCode; 200 > statusCode || statusCode >= 400 {
		return fmt.Errorf("http status %s", res.Status)
	}

	return nil
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace string, c *chart.Chart) error {
	return d.Write(ctx, u.JoinPath(namespace), c)
}
