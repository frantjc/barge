package util

import (
	"context"
	"io"
)

type stdoutContextKey struct{}

func StdoutInto(ctx context.Context, stdout io.Writer) context.Context {
	return context.WithValue(ctx, stdoutContextKey{}, stdout)
}

func StdoutFrom(ctx context.Context) io.Writer {
	v := ctx.Value(stdoutContextKey{})
	if v == nil {
		return io.Discard
	}

	switch v := v.(type) {
	case io.Writer:
		return v
	default:
		return io.Discard
	}
}

type stderrContextKey struct{}

func StderrInto(ctx context.Context, stderr io.Writer) context.Context {
	return context.WithValue(ctx, stderrContextKey{}, stderr)
}

func StderrFrom(ctx context.Context) io.Writer {
	v := ctx.Value(stderrContextKey{})
	if v == nil {
		return io.Discard
	}

	switch v := v.(type) {
	case io.Writer:
		return v
	default:
		return io.Discard
	}
}
