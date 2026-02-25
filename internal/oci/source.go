package oci

import (
	"bytes"
	"context"
	"net/url"

	"github.com/Masterminds/semver/v3"
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

func (s *source) Versions(ctx context.Context, u *url.URL, name string) ([]barge.SyncableVersion, error) {
	r, err := util.NewRegistryClientFromURL(ctx, u)
	if err != nil {
		return nil, err
	}

	tags, err := r.Tags(util.RefFromURL(u.JoinPath(name)))
	if err != nil {
		return nil, err
	}

	res := []barge.SyncableVersion{}
	for _, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		t := u.JoinPath(name)
		q := t.Query()
		q.Set("version", tag)
		t.RawQuery = q.Encode()
	
		w := barge.URL(*t)
		v := barge.Version(*version)

		res = append(res, barge.SyncableVersion{
			URL:     &w,
			Version: &v,
		})
	}

	return res, nil
}
