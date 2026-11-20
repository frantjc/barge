package credential

import (
	"context"

	"oras.land/oras-go/v2/registry/remote/credentials"
)

// Docker finds credentials that `docker login` stored for the Request URL's
// host, in $DOCKER_CONFIG/config.json or a credential helper that it names.
// Registries such as Artifactory, Harbor and Nexus often serve Helm charts
// from the same host, with the same credentials.
type Docker struct{}

func (Docker) String() string {
	return "docker"
}

func (Docker) Credential(ctx context.Context, req *Request) (*Credential, error) {
	store, err := credentials.NewStoreFromDocker(storeOptions)
	if err != nil {
		return nil, err
	}

	return credentialFromStore(ctx, store, req)
}

// HelmRegistry finds credentials that `helm registry login` stored for the
// Request URL's host, in Helm's registry config or a credential helper that
// it names.
type HelmRegistry struct {
	// Path returns the path to Helm's registry config.
	Path func() string
}

func (HelmRegistry) String() string {
	return "helm registry"
}

func (h HelmRegistry) Credential(ctx context.Context, req *Request) (*Credential, error) {
	if h.Path == nil {
		return nil, nil
	}

	path := h.Path()
	if path == "" {
		return nil, nil
	}

	// A missing file is fine, it's treated as empty.
	store, err := credentials.NewStore(path, storeOptions)
	if err != nil {
		return nil, err
	}

	return credentialFromStore(ctx, store, req)
}

// storeOptions match Helm's, so that barge finds the same credentials as
// Helm, including in the platform's default credential helper.
var storeOptions = credentials.StoreOptions{
	DetectDefaultNativeStore: true,
}

func credentialFromStore(ctx context.Context, store credentials.Store, req *Request) (*Credential, error) {
	cred, err := store.Get(ctx, req.URL.Host)
	if err != nil {
		return nil, err
	}

	switch {
	case cred.Username != "" && cred.Password != "":
		return &Credential{Username: cred.Username, Password: cred.Password}, nil
	case cred.RefreshToken != "":
		// An identity token from e.g. `az acr login`, only good for OCI.
		return &Credential{Username: cred.Username, RefreshToken: cred.RefreshToken}, nil
	case cred.AccessToken != "":
		return &Credential{Token: cred.AccessToken}, nil
	}

	return nil, nil
}
