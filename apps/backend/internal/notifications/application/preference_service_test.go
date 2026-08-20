package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// recordingAudit wraps auditapp.Noop and records every entry so preference
// tests can assert what ReplaceChannelPreferences wrote to the audit journal.
// WithTx returns the recorder itself so entries survive the tx binding the
// service performs inside runInTx.
type recordingAudit struct {
	auditapp.Noop
	entries []auditdomain.Entry
}

func (r *recordingAudit) WithTx(transaction.Tx) auditapp.Recorder { return r }

func (r *recordingAudit) Record(_ context.Context, e auditdomain.Entry) error {
	r.entries = append(r.entries, e)
	return nil
}

// preferenceHarness wires a PreferenceService to the shared fake repo, a
// recording audit and a fakeUoW so ReplaceChannelPreferences runs through
// runInTx (ADR 0033).
type preferenceHarness struct {
	repo  *fakeReminderRepo
	audit *recordingAudit
	b     *countingBeginner
	svc   *PreferenceService
}

func newPreferenceHarness() *preferenceHarness {
	repo := &fakeReminderRepo{}
	audit := &recordingAudit{}
	b := &countingBeginner{}
	f := NewTxStoreFactory(repo, audit, fakeUoW{beginner: b})
	return &preferenceHarness{repo: repo, audit: audit, b: b, svc: NewPreferenceService(f)}
}

// TestReplaceChannelPreferences_UpsertsInTxAndAuditsChanges proves the happy
// path runs inside one transaction: every submitted pair is upserted, exactly
// one audit entry is recorded when permissions change, and the transaction
// commits.
func TestReplaceChannelPreferences_UpsertsInTxAndAuditsChanges(t *testing.T) {
	h := newPreferenceHarness()
	userID := uuid.Must(uuid.NewV7())

	// Nothing stored yet: the effective set is the all-allowed default. The
	// submitted set flips one pair to denied.
	prefs := domain.DefaultNotificationChannelPreferences()
	prefs[0].Allowed = false

	got, err := h.svc.ReplaceChannelPreferences(t.Context(), userID, prefs)
	if err != nil {
		t.Fatalf("ReplaceChannelPreferences error = %v", err)
	}

	assertPreferenceUpserts(t, h, prefs)
	assertPreferenceAuditEntry(t, h, userID, prefs)
	assertEffectivePreferences(t, got, prefs)
	assertTxCounters(t, h, 1, 0, 0)
}

// assertPreferenceUpserts proves every submitted pair was upserted.
func assertPreferenceUpserts(t *testing.T, h *preferenceHarness, prefs []domain.NotificationChannelPreference) {
	t.Helper()
	if len(h.repo.upsertedPrefs) != len(prefs) {
		t.Fatalf("upserts = %d, want %d (every submitted pair)", len(h.repo.upsertedPrefs), len(prefs))
	}
}

// assertPreferenceAuditEntry proves exactly one audit entry landed, carrying
// the actor and entity ids and the single flipped change pair.
func assertPreferenceAuditEntry(t *testing.T, h *preferenceHarness, userID uuid.UUID, prefs []domain.NotificationChannelPreference) {
	t.Helper()
	if len(h.audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(h.audit.entries))
	}
	entry := h.audit.entries[0]
	if entry.Action != auditdomain.ActionNotificationPreferencesUpdated {
		t.Errorf("audit action = %s, want %s", entry.Action, auditdomain.ActionNotificationPreferencesUpdated)
	}
	if entry.ActorID == nil || *entry.ActorID != userID {
		t.Errorf("audit actor = %v, want %s", entry.ActorID, userID)
	}
	if entry.EntityType != auditdomain.EntityUser || entry.EntityID == nil || *entry.EntityID != userID {
		t.Errorf("audit entity = %s/%v, want user/%s", entry.EntityType, entry.EntityID, userID)
	}
	changes, ok := entry.Context["changes"].([]channelPreferenceChange)
	if !ok || len(changes) != 1 {
		t.Fatalf("audit changes = %#v, want the single flipped pair", entry.Context["changes"])
	}
	if changes[0].EventType != prefs[0].EventType || changes[0].Channel != prefs[0].Channel {
		t.Errorf("audit change pair = %s/%s, want %s/%s", changes[0].EventType, changes[0].Channel, prefs[0].EventType, prefs[0].Channel)
	}
	if !changes[0].Old || changes[0].New {
		t.Errorf("audit change old/new = %t/%t, want true/false", changes[0].Old, changes[0].New)
	}
}

// assertEffectivePreferences proves the effective set mirrors the submitted
// one, with the first pair denied.
func assertEffectivePreferences(t *testing.T, got, prefs []domain.NotificationChannelPreference) {
	t.Helper()
	if len(got) != len(prefs) {
		t.Fatalf("effective prefs = %d, want %d", len(got), len(prefs))
	}
	if got[0].Allowed {
		t.Errorf("effective pair %s/%s still allowed, want denied", got[0].EventType, got[0].Channel)
	}
}

// assertTxCounters proves the transaction counters match the expected
// committed/rolledBack/open outcome.
func assertTxCounters(t *testing.T, h *preferenceHarness, committed, rolledBack, open int) {
	t.Helper()
	if h.b.committed != committed || h.b.rolledBack != rolledBack || h.b.open != open {
		t.Errorf("tx counters committed/rolledBack/open = %d/%d/%d, want %d/%d/%d",
			h.b.committed, h.b.rolledBack, h.b.open, committed, rolledBack, open)
	}
}

// TestReplaceChannelPreferences_NoChangesSkipsAudit proves a set identical to
// the stored one is still upserted (idempotent replace) but produces no audit
// entry, matching the pre-UoW behavior.
func TestReplaceChannelPreferences_NoChangesSkipsAudit(t *testing.T) {
	h := newPreferenceHarness()
	userID := uuid.Must(uuid.NewV7())

	prefs := domain.DefaultNotificationChannelPreferences()
	got, err := h.svc.ReplaceChannelPreferences(t.Context(), userID, prefs)
	if err != nil {
		t.Fatalf("ReplaceChannelPreferences error = %v", err)
	}

	if len(h.repo.upsertedPrefs) != len(prefs) {
		t.Fatalf("upserts = %d, want %d (replace stays idempotent)", len(h.repo.upsertedPrefs), len(prefs))
	}
	if len(h.audit.entries) != 0 {
		t.Fatalf("audit entries = %d, want 0 for an unchanged set", len(h.audit.entries))
	}
	if len(got) != len(prefs) {
		t.Fatalf("effective prefs = %d, want %d", len(got), len(prefs))
	}
	if h.b.committed != 1 {
		t.Errorf("committed = %d, want 1", h.b.committed)
	}
}

// TestReplaceChannelPreferences_InvalidSetFailsBeforeTx proves a set that does
// not carry exactly one entry per (event type, channel) pair is rejected by
// validation before any transaction is opened.
func TestReplaceChannelPreferences_InvalidSetFailsBeforeTx(t *testing.T) {
	h := newPreferenceHarness()
	userID := uuid.Must(uuid.NewV7())

	_, err := h.svc.ReplaceChannelPreferences(t.Context(), userID, nil)
	if !errors.Is(err, ErrInvalidPreferences) {
		t.Fatalf("error = %v, want ErrInvalidPreferences", err)
	}
	if h.b.begun != 0 {
		t.Errorf("transactions begun = %d, want 0 (validation precedes runInTx)", h.b.begun)
	}
	if len(h.repo.upsertedPrefs) != 0 {
		t.Errorf("upserts = %d, want 0", len(h.repo.upsertedPrefs))
	}
}

// TestReplaceChannelPreferences_UpsertErrorRollsBackAndSkipsAudit proves a
// repository failure mid-replace rolls the transaction back and records no
// audit entry: the journal never describes a change that did not land.
func TestReplaceChannelPreferences_UpsertErrorRollsBackAndSkipsAudit(t *testing.T) {
	h := newPreferenceHarness()
	h.repo.upsertErr = errors.New("db unavailable")
	userID := uuid.Must(uuid.NewV7())

	prefs := domain.DefaultNotificationChannelPreferences()
	prefs[0].Allowed = false

	_, err := h.svc.ReplaceChannelPreferences(t.Context(), userID, prefs)
	if err == nil {
		t.Fatal("ReplaceChannelPreferences returned nil, want upsert error")
	}
	if want := "upsert notification channel preference"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to mention %q", err.Error(), want)
	}
	if len(h.audit.entries) != 0 {
		t.Fatalf("audit entries = %d, want 0 on rolled-back replace", len(h.audit.entries))
	}
	if h.b.committed != 0 || h.b.rolledBack != 1 || h.b.open != 0 {
		t.Errorf("tx counters committed/rolledBack/open = %d/%d/%d, want 0/1/0", h.b.committed, h.b.rolledBack, h.b.open)
	}
}
