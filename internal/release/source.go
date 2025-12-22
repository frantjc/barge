package file

import (
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterSource(
		new(source),
		"release",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	settings := barge.HelmSettings()
	cfg := new(action.Configuration)

	if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", nil); err != nil {
		return nil, err
	}

	release, err := action.NewGet(cfg).Run(u.Host)
	if err != nil {
		return nil, err
	}

	return release.Chart, nil
}
