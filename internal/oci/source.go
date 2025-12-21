package oci

import (
	"bytes"
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/utils"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func init() {
	barge.RegisterSource(
		new(source),
		"oci",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	r, err := utils.NewRegistryClientFromURL(u)
	if err != nil {
		return nil, err
	}

	ref := utils.RefFromURL(u)

	res, err := r.Pull(ref)
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(bytes.NewReader(res.Chart.Data))
}
