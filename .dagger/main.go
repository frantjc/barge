// A generated module for Sindri functions

package main

import (
	"context"
	"strings"

	"github.com/frantjc/barge/.dagger/internal/dagger"
	xslices "github.com/frantjc/x/slices"
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

func (m *BargeDev) Fmt(ctx context.Context) *dagger.Changeset {
	goModules := []string{
		".dagger/",
	}

	root := dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: goModules,
		}),
	}).
		Container().
		WithExec([]string{"go", "fmt", "./..."}).
		Directory(".")

	for _, module := range goModules {
		root = root.WithDirectory(
			module,
			dag.Go(dagger.GoOpts{
				Module: m.Source.Directory(module).Filter(dagger.DirectoryFilterOpts{
					Exclude: xslices.Filter(goModules, func(m string, _ int) bool {
						return strings.HasPrefix(m, module)
					}),
				}),
			}).
				Container().
				WithExec([]string{"go", "fmt", "./..."}).
				Directory("."),
		)
	}

	return root.Changes(m.Source)
}

func (m *BargeDev) Test(
	ctx context.Context,
	// +optional
	githubToken *dagger.Secret,
	// +optional
	githubRepo string,
) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module:                  m.Source,
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
		CombinedOutput(ctx)
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
		Module: m.Source,
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/barge",
			Ldflags: "-s -w -X main.version=" + version,
			Goos:    goos,
			Goarch:  goarch,
		})
}

func (m *BargeDev) Vulncheck(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{"go", "install", "golang.org/x/vuln/cmd/govulncheck@v1.1.4"}).
		WithExec([]string{"govulncheck", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Vet(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{"go", "vet", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Staticcheck(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{"go", "install", "honnef.co/go/tools/cmd/staticcheck@v0.6.1"}).
		WithExec([]string{"staticcheck", "./..."}).
		CombinedOutput(ctx)
}
