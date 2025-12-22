package barge

import (
	"context"
	"net/url"
	"sync"

	"helm.sh/helm/v3/pkg/chart"
)

type Destination interface {
	Write(context.Context, *url.URL, *chart.Chart) error
}

var (
	destMux = map[string]Destination{}
	destMu  sync.Mutex
)

func RegisterDestination(o Destination, scheme string, schemes ...string) {
	destMu.Lock()
	defer destMu.Unlock()

	for _, s := range append(schemes, scheme) {
		if _, ok := destMux[s]; ok {
			panic("attempt to reregister scheme: " + s)
		}

		destMux[s] = o
	}
}
