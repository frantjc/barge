package util

import (
	"bytes"
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

type stdinContextKey struct{}

func StdinInto(ctx context.Context, stdin io.Reader) context.Context {
	return context.WithValue(ctx, stdinContextKey{}, stdin)
}

// StdinFrom returns the stdin that StdinInto put into ctx, or else an empty
// reader, so that nothing reads the process's own stdin unless it was asked
// to.
func StdinFrom(ctx context.Context) io.Reader {
	v := ctx.Value(stdinContextKey{})
	if v == nil {
		return bytes.NewReader(nil)
	}

	switch v := v.(type) {
	case io.Reader:
		return v
	default:
		return bytes.NewReader(nil)
	}
}
