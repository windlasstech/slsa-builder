package npmprofile

import (
	"context"
	"io"
)

type operationalDebugOutputKey struct{}

// WithOperationalDebugOutput attaches a best-effort human diagnostic destination.
func WithOperationalDebugOutput(ctx context.Context, output io.Writer) context.Context {
	if output == nil {
		output = io.Discard
	}
	return context.WithValue(ctx, operationalDebugOutputKey{}, output)
}

func writeOperationalDebug(ctx context.Context, line string) {
	output, ok := ctx.Value(operationalDebugOutputKey{}).(io.Writer)
	if !ok {
		output = io.Discard
	}
	if _, err := io.WriteString(output, "windlass operational: "+line+"\n"); err != nil {
		return
	}
}
