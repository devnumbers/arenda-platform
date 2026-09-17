package notificationsjob

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// ErrorHandler logs delivery-job errors and panics through the process
// logger. River already turns a returned error into the retry ladder and a
// panic into a retry, so the handler only reports: no result is ever
// returned, the default retry behaviour stands.
type ErrorHandler struct {
	log *slog.Logger
}

// NewErrorHandler builds the River error handler.
func NewErrorHandler(log *slog.Logger) *ErrorHandler {
	if log == nil {
		log = slog.Default()
	}
	return &ErrorHandler{log: log}
}

// HandleError reports a failed job attempt. The error may carry recipient
// addresses (SMTP replies) — it goes through sanitize.
func (h *ErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	h.log.ErrorContext(ctx, "delivery job failed",
		slog.String("kind", job.Kind),
		slog.Int64("job_id", job.ID),
		slog.Int("attempt", job.Attempt),
		slog.String("state", string(job.State)),
		slog.String("error", sanitize.Error(err)))
	return nil
}

// HandlePanic reports a job worker panic; River retries the job with its
// default policy.
func (h *ErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, panicVal any, trace string) *river.ErrorHandlerResult {
	h.log.ErrorContext(ctx, "delivery job panicked",
		slog.String("kind", job.Kind),
		slog.Int64("job_id", job.ID),
		slog.Int("attempt", job.Attempt),
		slog.Any("panic", panicVal),
		slog.String("trace", trace))
	return nil
}
