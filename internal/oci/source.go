package oci

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
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
	r, err := util.NewRegistryClientFromURL(ctx, u)
	if err != nil {
		return nil, err
	}

	ref := util.RefFromURL(u)

	res, err := r.Pull(ref)
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(bytes.NewReader(res.Chart.Data))
}

func (s *source) Versions(ctx context.Context, u *url.URL, name string) ([]string, error) {
	r, err := util.NewRegistryClientFromURL(ctx, u)
	if err != nil {
		return nil, err
	}

	ref, _, _ := strings.Cut(util.RefFromURL(u.JoinPath(name)), ":")

	return r.Tags(ref)
}
