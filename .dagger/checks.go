package main

import (
	"context"
	"fmt"
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
	oci []string,
) error {
	test, err := m.Test(ctx, oci)
	if err != nil {
		return err
	}

	if _, err = test.CombinedOutput(ctx); err != nil {
		return err
	}

	return nil
}
