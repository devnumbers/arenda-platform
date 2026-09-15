// Package logger provides slog.Handler factories for different output formats.
//
// JSON output remains the default for stage and production to keep structured
// log aggregation unchanged. A colored "pretty" handler is used for local and
// dev environments to make console output easier to read.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"
)

// Format names.
const (
	FormatJSON   = "json"
	FormatPretty = "pretty"
)

// NewHandler returns a slog.Handler for the requested format and level.
// Supported formats are "json" and "pretty". Any other value returns an error.
func NewHandler(format string, level slog.Level, out io.Writer) (slog.Handler, error) {
	opts := &slog.HandlerOptions{Level: level}

	switch format {
	case FormatJSON:
		return slog.NewJSONHandler(out, opts), nil
	case FormatPretty:
		return tint.NewHandler(out, &tint.Options{
			Level:      level,
			TimeFormat: time.TimeOnly,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q: must be %q or %q", format, FormatJSON, FormatPretty)
	}
}
