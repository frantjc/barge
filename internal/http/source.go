package http

import (
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/utils"
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
	scheme := u.Scheme

	g, err := getter.All(settings).ByScheme(scheme)
	if err != nil {
		return nil, err
	}

	opts := []getter.Option{}

	if username, password, ok := utils.UsernameAndPasswordForURLWithEnvFallback(u, utils.LocationSource, scheme); ok {
		opts = append(opts, getter.WithBasicAuth(username, password))
	}

	buf, err := g.Get(u.String(), opts...)
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(buf)
}
