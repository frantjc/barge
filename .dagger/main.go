package main

import (
	"context"

	"github.com/frantjc/barge/.dagger/internal/dagger"
)

type BargeDev struct {
	Source *dagger.Directory
}

func New(
	ctx context.Context,
	// +optional
	// +defaultPath="."
	src *dagger.Directory,
) (*BargeDev, error) {
	return &BargeDev{
		Source: src,
	}, nil
}

// +check
func (m *BargeDev) Test(
	ctx context.Context,
	// +optional
	githubToken *dagger.Secret,
	// +optional
	githubRepo string,
) error {
	if _, err := dag.Go(dagger.GoOpts{
		Source:                  m.Source,
		AdditionalWolfiPackages: []string{"helm-4"},
	}).
		Container().
		With(func(r *dagger.Container) *dagger.Container {
			if githubToken != nil {
				r = r.
					WithSecretVariable("GITHUB_TOKEN", githubToken)
			}
			if githubRepo != "" {
				r = r.
					WithEnvVariable("GITHUB_REPOSITORY", githubRepo)
			}
			return r
		}).
		WithExec([]string{"go", "test", "-race", "-cover", "-test.v", "./..."}, dagger.ContainerWithExecOpts{ExperimentalPrivilegedNesting: true}).
		Sync(ctx); err != nil {
		return err
	}

	return nil
}

func (m *BargeDev) Release(
	ctx context.Context,
	githubRepo string,
	githubToken *dagger.Secret,
) error {
	return dag.Release(
		m.Source.AsGit().LatestVersion(),
	).
		Create(ctx, githubToken, githubRepo, "barge", dagger.ReleaseCreateOpts{Brew: true})
}

func (m *BargeDev) Binary(
	ctx context.Context,
	// +default=v0.0.0-unknown
	version string,
	// +optional
	goarch string,
	// +optional
	goos string,
) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Source: m.Source,
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/barge",
			Ldflags: "-s -w -X main.version=" + version,
			Goos:    goos,
			Goarch:  goarch,
		})
}
