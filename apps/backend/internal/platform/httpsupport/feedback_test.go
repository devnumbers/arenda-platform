package httpsupport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// captureMailer records the message the handler hands to the mailer port.
type captureMailer struct {
	msg mailer.Message
	err error
}

func (m *captureMailer) Send(_ context.Context, msg mailer.Message) error {
	m.msg = msg
	return m.err
}

func doFeedbackJSON(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func TestFeedbackHandlersSendFeedback(t *testing.T) {
	t.Parallel()

	t.Run("sends the question to the configured recipient", func(t *testing.T) {
		t.Parallel()
		sender := &captureMailer{}
		h := NewFeedbackHandlers(sender, "owner@example.com", nil)

		rr := doFeedbackJSON(t, h.SendFeedback, `{"email":"user@example.com","message":"Как оплатить тариф?"}`)

		require.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, []string{"owner@example.com"}, sender.msg.To)
		assert.Equal(t, "Вопрос с лендинга Рентли", sender.msg.Subject)
		assert.Contains(t, sender.msg.TextBody, "user@example.com")
		assert.Contains(t, sender.msg.TextBody, "Как оплатить тариф?")
	})

	t.Run("normalizes the sender email", func(t *testing.T) {
		t.Parallel()
		sender := &captureMailer{}
		h := NewFeedbackHandlers(sender, "owner@example.com", nil)

		rr := doFeedbackJSON(t, h.SendFeedback, `{"email":"  USER@Example.COM ","message":"вопрос"}`)

		require.Equal(t, http.StatusNoContent, rr.Code)
		assert.Contains(t, sender.msg.TextBody, "user@example.com")
	})

	t.Run("redacts sensitive data from the message body", func(t *testing.T) {
		t.Parallel()
		sender := &captureMailer{}
		h := NewFeedbackHandlers(sender, "owner@example.com", nil)

		rr := doFeedbackJSON(t, h.SendFeedback, `{"email":"user@example.com","message":"Мой телефон +79991234567, вопрос про тариф"}`)

		require.Equal(t, http.StatusNoContent, rr.Code)
		assert.Contains(t, sender.msg.TextBody, "[REDACTED]")
		assert.NotContains(t, sender.msg.TextBody, "+79991234567")
	})

	validationCases := []struct {
		name string
		body string
		want int
	}{
		{"invalid email", `{"email":"not-an-email","message":"вопрос"}`, http.StatusBadRequest},
		{"empty email", `{"email":"","message":"вопрос"}`, http.StatusBadRequest},
		{"missing email", `{"message":"вопрос"}`, http.StatusBadRequest},
		{"blank message", `{"email":"user@example.com","message":"   "}`, http.StatusBadRequest},
		{"missing message", `{"email":"user@example.com"}`, http.StatusBadRequest},
		{"oversized message", `{"email":"user@example.com","message":"` + strings.Repeat("а", 1001) + `"}`, http.StatusBadRequest},
		{"malformed json", `{"email":`, http.StatusBadRequest},
	}
	for _, tc := range validationCases {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewFeedbackHandlers(&captureMailer{}, "owner@example.com", nil)

			rr := doFeedbackJSON(t, h.SendFeedback, tc.body)

			require.Equal(t, tc.want, rr.Code)
		})
	}

	t.Run("rate limits per client", func(t *testing.T) {
		t.Parallel()
		limiter := NewRateLimiter(rate.Every(time.Minute), 1, time.Hour)
		h := NewFeedbackHandlers(&captureMailer{}, "owner@example.com", limiter)

		first := doFeedbackJSON(t, h.SendFeedback, `{"email":"user@example.com","message":"вопрос"}`)
		require.Equal(t, http.StatusNoContent, first.Code)

		second := doFeedbackJSON(t, h.SendFeedback, `{"email":"user@example.com","message":"вопрос"}`)
		require.Equal(t, http.StatusTooManyRequests, second.Code)
	})

	t.Run("answers 500 when the mailer fails", func(t *testing.T) {
		t.Parallel()
		sender := &captureMailer{err: errors.New("smtp unavailable")}
		h := NewFeedbackHandlers(sender, "owner@example.com", nil)

		rr := doFeedbackJSON(t, h.SendFeedback, `{"email":"user@example.com","message":"вопрос"}`)

		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("answers 503 when no recipient is configured", func(t *testing.T) {
		t.Parallel()
		h := NewFeedbackHandlers(&captureMailer{}, "", nil)

		rr := doFeedbackJSON(t, h.SendFeedback, `{"email":"user@example.com","message":"вопрос"}`)

		require.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})
}
