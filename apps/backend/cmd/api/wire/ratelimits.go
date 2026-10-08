package wire

import (
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"golang.org/x/time/rate"
)

// RateLimiters holds the rate limiters wired by WireRateLimiters: the
// per-endpoint HTTP limiters plus the anti-flood send limits (#1210)
// consulted inside LoginCodeService.
type RateLimiters struct {
	IPRateLimiter            *httpsupport.RateLimiter
	EmailSendLimiter         *httpsupport.RateLimiter
	EmailVerifyLimiter       *httpsupport.RateLimiter
	PhoneChangeSendLimiter   *httpsupport.RateLimiter
	PhoneChangeVerifyLimiter *httpsupport.RateLimiter
	EmailChangeSendLimiter   *httpsupport.RateLimiter
	// RecipientSendLimiter and InitiatorSendLimiter are the anti-flood send
	// limits (#1210) consulted inside LoginCodeService on each actual code
	// issuance: per recipient email and per authenticated initiator, across
	// all flows. Same in-memory token-bucket class as every limiter here —
	// the restart resets the windows, which the e2e harness relies on.
	RecipientSendLimiter *httpsupport.RateLimiter
	InitiatorSendLimiter *httpsupport.RateLimiter
	ClientErrorsLimiter  *httpsupport.RateLimiter
	FeedbackLimiter      *httpsupport.RateLimiter
}

// WireRateLimiters constructs the rate limiters from the config. The
// returned Stop() helper stops every limiter and must be deferred by the caller.
func WireRateLimiters(cfg *config.Config) *RateLimiters {
	ipLimiter := httpsupport.NewRateLimiter(rate.Limit(cfg.RateLimit.IPRPS), cfg.RateLimit.IPBurst, 1*time.Hour)

	emailSendBurst := min(3, cfg.RateLimit.EmailSendPerHour)
	emailSendLimiter := httpsupport.NewRateLimiter(rate.Limit(cfg.RateLimit.EmailSendPerHour)/3600, emailSendBurst, 1*time.Hour)

	emailVerifyBurst := min(5, cfg.RateLimit.EmailVerifyPer15Min)
	emailVerifyLimiter := httpsupport.NewRateLimiter(rate.Limit(cfg.RateLimit.EmailVerifyPer15Min)/(15*60), emailVerifyBurst, 1*time.Hour)

	phoneChangeSendBurst := min(3, cfg.RateLimit.PhoneChangeSendPerHour)
	phoneChangeSendLimiter := httpsupport.NewRateLimiter(
		rate.Every(time.Hour/time.Duration(cfg.RateLimit.PhoneChangeSendPerHour)), phoneChangeSendBurst, 1*time.Hour)

	phoneChangeVerifyBurst := min(5, cfg.RateLimit.PhoneChangeVerifyPer15Min)
	phoneChangeVerifyLimiter := httpsupport.NewRateLimiter(
		rate.Every(15*time.Minute/time.Duration(cfg.RateLimit.PhoneChangeVerifyPer15Min)), phoneChangeVerifyBurst, 1*time.Hour)

	// The per-user budget on email-change sends to NEW addresses (step 2);
	// grilling decision #720-3 pins it at 5/hour, mirroring the phone change.
	emailChangeSendBurst := min(3, cfg.RateLimit.EmailChangeSendPerHour)
	emailChangeSendLimiter := httpsupport.NewRateLimiter(
		rate.Every(time.Hour/time.Duration(cfg.RateLimit.EmailChangeSendPerHour)), emailChangeSendBurst, 1*time.Hour)

	// The anti-flood send limits (#1210), consumed inside LoginCodeService on
	// each actual issuance: 5 codes per hour to one recipient address, 10 per
	// hour per authenticated initiator — the triple throttle alone leaves a
	// drip of 60 letters an hour into a victim's mailbox.
	recipientSendLimiter := httpsupport.NewRateLimiter(
		rate.Every(time.Hour/time.Duration(cfg.RateLimit.CodeSendPerRecipientPerHour)),
		min(3, cfg.RateLimit.CodeSendPerRecipientPerHour), 1*time.Hour)
	initiatorSendLimiter := httpsupport.NewRateLimiter(
		rate.Every(time.Hour/time.Duration(cfg.RateLimit.CodeSendPerInitiatorPerHour)),
		min(3, cfg.RateLimit.CodeSendPerInitiatorPerHour), 1*time.Hour)

	clientErrorsLimiter := httpsupport.NewRateLimiter(rate.Every(2*time.Second), 10, time.Minute)

	// The feedback form (карта #1010) emails a personal mailbox from a public
	// unauthenticated endpoint: a tight per-IP budget — three quick attempts,
	// then one per 30 seconds.
	feedbackLimiter := httpsupport.NewRateLimiter(rate.Every(30*time.Second), 3, time.Hour)

	return &RateLimiters{
		IPRateLimiter:            ipLimiter,
		EmailSendLimiter:         emailSendLimiter,
		EmailVerifyLimiter:       emailVerifyLimiter,
		PhoneChangeSendLimiter:   phoneChangeSendLimiter,
		PhoneChangeVerifyLimiter: phoneChangeVerifyLimiter,
		EmailChangeSendLimiter:   emailChangeSendLimiter,
		RecipientSendLimiter:     recipientSendLimiter,
		InitiatorSendLimiter:     initiatorSendLimiter,
		ClientErrorsLimiter:      clientErrorsLimiter,
		FeedbackLimiter:          feedbackLimiter,
	}
}

// Stop stops every rate limiter. It is safe to call once on shutdown.
func (r *RateLimiters) Stop() {
	r.IPRateLimiter.Stop()
	r.EmailSendLimiter.Stop()
	r.EmailVerifyLimiter.Stop()
	r.PhoneChangeSendLimiter.Stop()
	r.PhoneChangeVerifyLimiter.Stop()
	r.EmailChangeSendLimiter.Stop()
	r.RecipientSendLimiter.Stop()
	r.InitiatorSendLimiter.Stop()
	r.ClientErrorsLimiter.Stop()
	r.FeedbackLimiter.Stop()
}
