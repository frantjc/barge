package barge

import (
	"context"
	"net/url"
	"sync"

	"github.com/Masterminds/semver/v3"
	"helm.sh/helm/v3/pkg/chart"
)

type Chart chart.Chart

func (j Chart) Name() string {
	c := chart.Chart(j)
	return c.Name()
}

type Source interface {
	Open(context.Context, *url.URL) (*chart.Chart, error)
}

var (
	srcMux = map[string]Source{}
	srcMu  sync.Mutex
)

func RegisterSource(o Source, scheme string, schemes ...string) {
	srcMu.Lock()
	defer srcMu.Unlock()

	for _, s := range append(schemes, scheme) {
		if _, ok := srcMux[s]; ok {
			panic("attempt to reregister scheme: " + s)
		}

		srcMux[s] = o
	}
}

type Version semver.Version

func (j Version) GreaterThan(version *Version) bool {
	i := semver.Version(j)
	v := semver.Version(*version)
	return i.GreaterThan(&v)
}

type SyncableVersion struct {
	URL     *URL
	Version *Version
}

type SyncableSource interface {
	Versions(context.Context, *url.URL, string) ([]SyncableVersion, error)
}
