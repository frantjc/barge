package http

import (
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
)

func init() {
	barge.RegisterSource(
		new(source),
		"http",
		"https",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	settings := barge.HelmSettings()

	g, err := getter.All(settings).ByScheme(u.Scheme)
	if err != nil {
		return nil, err
	}

	buf, err := g.Get(u.String())
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(buf)
}
