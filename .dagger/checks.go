package main

import (
	"context"
	"fmt"

	"github.com/frantjc/barge/.dagger/internal/dagger"
)

// +check
func (m *BargeDev) IsFmted(ctx context.Context) error {
	if empty, err := m.Fmt(ctx).IsEmpty(ctx); err != nil {
		return err
	} else if !empty {
		return fmt.Errorf("source is not formatted (run `dagger call fmt`)")
	}

	return nil
}

// +check
func (m *BargeDev) TestsPass(
	ctx context.Context,
	// +optional
	githubActor string,
	// +optional
	githubToken *dagger.Secret,
) error {
	oci := []string{}
	if githubToken != nil && githubActor != "" {
		oci = append(oci, fmt.Sprintf("ghcr.io/%s/barge/charts/test", githubActor))
	}
	test, err := m.Test(ctx, oci, githubActor, githubToken)
	if err != nil {
		return err
	}

	if _, err = test.CombinedOutput(ctx); err != nil {
		return err
	}

	return nil
}
