package barge

import (
	"context"
	"fmt"
	"net/url"
)

func Copy(ctx context.Context, src, dest string) error {
	if err := cp(ctx, src, dest); err != nil {
		return fmt.Errorf("barge: %v", err)
	}

	return nil
}

func cp(ctx context.Context, src, dest string) error {
	s, err := url.Parse(src)
	if err != nil {
		return err
	}

	if s.Scheme == "" {
		s.Scheme = "file"
	}

	source, ok := srcMux[s.Scheme]
	if !ok {
		return fmt.Errorf("no source registered for scheme: %s", s.Scheme)
	}

	d, err := url.Parse(dest)
	if err != nil {
		return err
	}

	if d.Scheme == "" {
		d.Scheme = "file"
	}

	destination, ok := destMux[d.Scheme]
	if !ok {
		return fmt.Errorf("no destination registered for scheme: %s", d.Scheme)
	}

	c, err := source.Open(ctx, s)
	if err != nil {
		return err
	}

	return destination.Write(ctx, d, c)
}
