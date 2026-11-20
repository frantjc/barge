package oci

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func init() {
	barge.RegisterSource(
		new(source),
		"oci",
		"oci+https",
		"oci+http",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	r, err := newRegistryClient(ctx, u, credential.Source)
	if err != nil {
		return nil, err
	}

	// Like `helm pull` without --version, pull the highest semver tag unless the
	// URL names one, via a tag in its path or its version query parameter.
	// Fall back to :latest if there's no semver tag, e.g. for charts that
	// barge pushed before it tagged charts with their version.
	w := *u
	if q := w.Query(); !strings.Contains(w.Path, ":") && q.Get("version") == "" {
		if tags, err := r.Tags(util.RefFromURL(&w)); err == nil && len(tags) > 0 {
			q.Set("version", tags[0])
			w.RawQuery = q.Encode()
		}
	}

	ref := util.RefFromURL(&w)

	res, err := r.Pull(ref)
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(bytes.NewReader(res.Chart.Data))
}

func (s *source) Versions(ctx context.Context, u *url.URL, name string) ([]barge.SyncableVersion, error) {
	r, err := newRegistryClient(ctx, u, credential.Source)
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
