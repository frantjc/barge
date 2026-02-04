package barge

import (
	"context"
	"net/url"
	"sync"

	"helm.sh/helm/v3/pkg/chart"
)

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

type QueryableSource interface {
	Versions(context.Context, *url.URL, string) ([]string, error)
}
