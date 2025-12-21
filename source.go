package barge

import (
	"context"
	"net/url"

	"helm.sh/helm/v3/pkg/chart"
)

type Source interface {
	Open(context.Context, *url.URL) (*chart.Chart, error)
}

var (
	srcMux = map[string]Source{}
)

func RegisterSource(o Source, scheme string, schemes ...string) {
	for _, s := range append(schemes, scheme) {
		if _, ok := srcMux[s]; ok {
			panic("attempt to reregister scheme: " + s)
		}

		srcMux[s] = o
	}
}
