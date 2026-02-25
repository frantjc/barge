package directory

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func init() {
	barge.RegisterSource(
		new(source),
		"directory",
		"dir",
		"source",
		"src",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	return loader.LoadDir(filepath.Join(u.Host, u.Path))
}

func (s *source) Versions(ctx context.Context, u *url.URL, name string) ([]barge.SyncableVersion, error) {
	root := filepath.Join(u.Host, u.Path)
	if fi, err := os.Stat(root); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}

	paths, err := filepath.Glob(filepath.Join(root, fmt.Sprintf("%s-*.tgz", name)))
	if err != nil {
		return nil, err
	}

	res := []barge.SyncableVersion{}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}

		rawVersion := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), fmt.Sprintf("%s-", name)), ".tgz")

		version, err := semver.NewVersion(rawVersion)
		if err != nil {
			continue
		}

		t := u.JoinPath(rel)
		t.Scheme = "file"
		w := barge.URL(*t)
		v := barge.Version(*version)

		res = append(res, barge.SyncableVersion{
			Version: &v,
			URL:     &w,
		})
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Version.GreaterThan(res[j].Version)
	})

	return res, nil
}
