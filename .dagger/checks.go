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
	githubToken *dagger.Secret,
	// +optional
	githubRepo string,
) error {
	if _, err := m.Test(ctx, githubToken, githubRepo); err != nil {
		return err
	}

	return nil
}
