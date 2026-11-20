package credential

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"strings"

	"helm.sh/helm/v3/pkg/repo"
)

// HelmRepositories finds credentials in Helm's repositories file, i.e. the ones
// given to `helm repo add --username --password`.
//
// The repository whose URL is the longest prefix of the Request's URL wins.
// Failing that, a repository on the same host is used, but only if every
// repository on that host has the same credentials. This handles e.g.
// Artifactory, whose Helm repositories are read from
// /artifactory/api/helm/<repo> but written to /artifactory/<repo>.
type HelmRepositories struct {
	// Path returns the path to the repositories file.
	Path func() string
}

func (HelmRepositories) String() string {
	return "helm repositories"
}

func (h HelmRepositories) Credential(_ context.Context, req *Request) (*Credential, error) {
	if h.Path == nil {
		return nil, nil
	}

	path := h.Path()
	if path == "" {
		return nil, nil
	}

	f, err := repo.LoadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var (
		host      = hostPort(req.URL)
		reqPath   = strings.TrimSuffix(req.URL.Path, "/")
		best      *Credential
		bestLen   = -1
		onHost    = map[Credential]struct{}{}
		onHostAny *Credential
	)

	for _, entry := range f.Repositories {
		if entry == nil || entry.Password == "" {
			continue
		}

		u, err := url.Parse(entry.URL)
		if err != nil || hostPort(u) != host {
			continue
		}

		cred := Credential{Username: entry.Username, Password: entry.Password}
		onHost[cred] = struct{}{}
		onHostAny = &cred

		entryPath := strings.TrimSuffix(u.Path, "/")
		if (reqPath == entryPath || strings.HasPrefix(reqPath, entryPath+"/")) && len(entryPath) > bestLen {
			best = &cred
			bestLen = len(entryPath)
		}
	}

	if best != nil {
		return best, nil
	}

	if len(onHost) == 1 {
		return onHostAny, nil
	}

	return nil, nil
}
