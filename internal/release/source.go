package file

import (
	"context"
	"fmt"
	"net/url"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
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
	namespace := u.Query().Get("namespace")
	if namespace == "" {
		namespace = settings.Namespace()
	}
	log := util.SloggerFrom(ctx)
	debug := func(format string, v ...interface{}) { log.Debug(fmt.Sprintf(format, v...)) }

	if err := cfg.Init(settings.RESTClientGetter(), namespace, "secret", debug); err != nil {
		return nil, err
	}

	release, err := action.NewGet(cfg).Run(u.Host)
	if err != nil {
		return nil, err
	}

	return release.Chart, nil
}
