package httpsupport

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// feedbackMessageMaxLength caps the question text; the landing modal enforces
// the same limit client-side, this is the authoritative check.
const (
	feedbackMessageMaxLength = 1000
	feedbackEmailMaxLength   = 254
)

var errInvalidFeedbackEmail = errors.New("invalid feedback email")

// FeedbackHandlers implements the public landing feedback endpoint
// (POST /feedback, карта #1010): the landing's server-side proxy forwards
// the «Задать вопрос» modal form, the question is emailed to the configured
// FEEDBACK_EMAIL recipient. Nothing is persisted.
type FeedbackHandlers struct {
	mailer    mailer.Sender
	recipient string
	limiter   *RateLimiter
}

// NewFeedbackHandlers creates handlers for POST /feedback. An empty
// recipient keeps the endpoint answering 503 until FEEDBACK_EMAIL is set.
func NewFeedbackHandlers(sender mailer.Sender, recipient string, limiter *RateLimiter) *FeedbackHandlers {
	return &FeedbackHandlers{mailer: sender, recipient: recipient, limiter: limiter}
}

// SendFeedback implements POST /feedback.
func (h *FeedbackHandlers) SendFeedback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.recipient == "" {
		LoggerFromContext(ctx).ErrorContext(ctx, "feedback recipient is not configured")
		WriteProblem(ctx, w, http.StatusServiceUnavailable, Problem(ctx, "Service unavailable", "Обратная связь временно недоступна"))
		return
	}
	clientIP := requestctx.ClientIPFromContext(ctx)
	if h.limiter != nil && !h.limiter.Allow(clientIP) {
		WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	var body openapi.FeedbackRequest
	if err := DecodeJSONBody(w, r, &body); err != nil {
		LoggerFromContext(ctx).WarnContext(ctx, "failed to decode feedback request", slog.String("error", SanitizeError(err)))
		WriteProblem(ctx, w, http.StatusBadRequest, Problem(ctx, "Bad request", "Некорректное тело запроса"))
		return
	}
	email, err := normalizeFeedbackEmail(body.Email)
	if err != nil {
		WriteProblem(ctx, w, http.StatusBadRequest, Problem(ctx, "Bad request", "Некорректный адрес электронной почты"))
		return
	}
	message := strings.TrimSpace(body.Message)
	if message == "" || utf8.RuneCountInString(message) > feedbackMessageMaxLength {
		WriteProblem(ctx, w, http.StatusBadRequest, Problem(ctx, "Bad request", "Сообщение пустое или слишком длинное"))
		return
	}

	// The body goes to a personal mailbox: sanitize first so pasted
	// credentials, phones, and card numbers never leave the perimeter.
	msg := mailer.Message{
		To:       []string{h.recipient},
		Subject:  "Вопрос с лендинга Рентли",
		TextBody: fmt.Sprintf("Новый вопрос с лендинга Рентли.\n\nОт: %s\n\n%s", email, sanitize.String(message)),
	}
	if err := h.mailer.Send(ctx, msg); err != nil {
		LoggerFromContext(ctx).ErrorContext(ctx, "failed to send feedback email", slog.String("error", SanitizeError(err)))
		WriteProblem(ctx, w, http.StatusInternalServerError, Problem(ctx, "Internal server error", "Не удалось отправить сообщение"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// normalizeFeedbackEmail trims and lowercases the sender address and checks
// it parses as a bare email (no display-name form). The bounded-context
// email validator (identity/domain) is off-limits from platform code, so
// this uses the stdlib parser.
func normalizeFeedbackEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || utf8.RuneCountInString(email) > feedbackEmailMaxLength {
		return "", errInvalidFeedbackEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", errInvalidFeedbackEmail
	}
	return email, nil
}
