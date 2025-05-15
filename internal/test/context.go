package test

import (
	"context"
	"testing"
	"time"
)

// ContextWithTimeout returns a context with a timeout that is cancelled when the test ends.
func ContextWithTimeout(tb testing.TB, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	tb.Cleanup(cancel)
	return ctx
}
