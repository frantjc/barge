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
	cluster := dag.Kwok().Cluster()
	alias := "kwok"
	tags := []string{"dagger", "examples", "kubernetes"}
	return dag.Go(dagger.GoOpts{
		Source: m.Source,
		Container: dag.Mise(dagger.MiseOpts{
			Source: m.Source,
		}).
			Container(dagger.MiseContainerOpts{
				Tools: []string{"go", "helm"},
			}).
			With(func(r *dagger.Container) *dagger.Container {
				if githubToken != nil && githubRepo != "" {
					tags = append(tags, "ghcr")
					return r.
						WithSecretVariable("GITHUB_TOKEN", githubToken).
						WithEnvVariable("GITHUB_REPOSITORY", githubRepo)
				}
				return r
			}).
			WithServiceBinding(alias, cluster.Container().AsService()).
			WithEnvVariable("KUBECONFIG", "$HOME/.kube/config", dagger.ContainerWithEnvVariableOpts{
				Expand: true,
			}).
			WithFile("$KUBECONFIG", cluster.KubeConfig(dagger.KwokClusterKubeConfigOpts{Alias: alias}), dagger.ContainerWithFileOpts{
				Expand: true,
			}),
	}).
		Test(ctx, dagger.GoTestOpts{
			Race: true,
			Tags: tags,
		})
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

// +check
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
