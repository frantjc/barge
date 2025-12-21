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

func (m *BargeDev) Test(ctx context.Context) (string, error) {
	chartmuseum := dag.Container().
		From("ghcr.io/helm/chartmuseum:v0.16.3").
		WithExposedPort(8080).
		WithEnvVariable("DEBUG", "1").
		WithEnvVariable("STORAGE", "local").
		WithEnvVariable("STORAGE_LOCAL_ROOTDIR", "/data").
		AsService()

	registry := dag.Container().
		From("docker.io/distribution/distribution:3").
		WithExposedPort(5000).
		AsService()

	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithServiceBinding("chartmuseum", chartmuseum).
		WithServiceBinding("registry", registry).
		WithEnvVariable("TEST_BARGE_OCI", "http://registry:5000/test").
		WithExec([]string{"go", "test", "-race", "-cover", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Version(ctx context.Context) string {
	version := "v0.0.0-unknown"

	gitRef := m.Source.AsGit().LatestVersion()

	if ref, err := gitRef.Ref(ctx); err == nil {
		version = strings.TrimPrefix(ref, "refs/tags/")
	}

	if latestVersionCommit, err := gitRef.Commit(ctx); err == nil {
		if headCommit, err := m.Source.AsGit().Head().Commit(ctx); err == nil {
			if headCommit != latestVersionCommit {
				if len(headCommit) > 7 {
					headCommit = headCommit[:7]
				}
				version += "-" + headCommit
			}
		}
	}

	if empty, _ := m.Source.AsGit().Uncommitted().IsEmpty(ctx); !empty {
		version += "+dirty"
	}

	return version
}

func (m *BargeDev) Tag(ctx context.Context) string {
	before, _, _ := strings.Cut(strings.TrimPrefix(m.Version(ctx), "v"), "+")
	return before
}

func (m *BargeDev) Binary(ctx context.Context) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{".github/", "e2e/"},
		}),
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/barge",
			Ldflags: "-s -w -X main.version=" + m.Version(ctx),
		})
}

func (m *BargeDev) Vulncheck(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{
				".dagger/",
			},
		}),
	}).
		Container().
		WithExec([]string{"go", "install", "golang.org/x/vuln/cmd/govulncheck@v1.1.4"}).
		WithExec([]string{"govulncheck", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Vet(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{
				".dagger/",
			},
		}),
	}).
		Container().
		WithExec([]string{"go", "vet", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Staticcheck(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{
				".dagger/",
			},
		}),
	}).
		Container().
		WithExec([]string{"go", "install", "honnef.co/go/tools/cmd/staticcheck@v0.6.1"}).
		WithExec([]string{"staticcheck", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Coder(ctx context.Context) (*dagger.LLM, error) {
	gopls := dag.Go(dagger.GoOpts{Module: m.Source}).
		Container().
		WithExec([]string{"go", "install", "golang.org/x/tools/gopls@latest"})

	instructions, err := gopls.WithExec([]string{"gopls", "mcp", "-instructions"}).Stdout(ctx)
	if err != nil {
		return nil, err
	}

	return dag.Doug().
		Agent(
			dag.LLM().
				WithEnv(
					dag.Env().
						WithCurrentModule().
						WithWorkspace(m.Source.Filter(dagger.DirectoryFilterOpts{
							Exclude: []string{".dagger/", ".github/"},
						})),
				).
				WithBlockedFunction("BargeDev", "tag").
				WithBlockedFunction("BargeDev", "version").
				WithSystemPrompt(instructions).
				WithMCPServer(
					"gopls",
					gopls.AsService(dagger.ContainerAsServiceOpts{
						Args: []string{"gopls", "mcp"},
					}),
				),
		), nil
}
