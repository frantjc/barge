package main

import (
	"context"

	"github.com/frantjc/barge/.dagger/internal/dagger"
)

type BargeDev struct{}

// +check
func (m *BargeDev) Test(
	ctx context.Context,
	workspace *dagger.Workspace,
	// +optional
	githubToken *dagger.Secret,
	// +optional
	githubRepo string,
) error {
	cluster := dag.Kwok().Cluster()
	alias := "kwok"
	tags := []string{"dagger", "examples", "kubernetes"}
	return dag.Go(dagger.GoOpts{
		Workspace: workspace,
		Container: dag.Mise(dagger.MiseOpts{
			Workspace: workspace,
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
	workspace *dagger.Workspace,
	githubRepo string,
	githubToken *dagger.Secret,
) error {
	return dag.Release(
		workspace.Directory(".").AsGit().LatestVersion(),
	).
		Create(ctx, githubToken, githubRepo, "barge", dagger.ReleaseCreateOpts{Brew: true})
}
