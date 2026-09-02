package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// MaxTitleLength bounds the rule title and MaxCommentLength the comment,
// counted in characters — product copy limits, not byte limits.
const (
	MaxTitleLength   = 255
	MaxCommentLength = 1000
)

// CreateRuleCommand is the validated create payload of a task rule. Creating
// a task is creating a rule: the first materialization (today's task, the
// single future or the undated one) happens in the same transaction.
type CreateRuleCommand struct {
	Title   string
	Comment *string
	DueDate *time.Time
	DueTime *domain.TimeOfDay
	Repeat  domain.RepeatKind
}

// CommentUpdate is the PATCH resolution of comment: Value nil clears it.
type CommentUpdate struct{ Value *string }

// DueDateUpdate is the PATCH resolution of dueDate: Value nil clears the
// date (the rule becomes undated; the repeat must be once).
type DueDateUpdate struct{ Value *time.Time }

// DueTimeUpdate is the PATCH resolution of dueTime: Value nil clears the
// time (the date stays).
type DueTimeUpdate struct{ Value *domain.TimeOfDay }

// UpdateRuleCommand is the partial-update payload: a nil field leaves the
// rule unchanged (omit = no change); the optional fields are tri-state.
type UpdateRuleCommand struct {
	Title   *string
	Comment *CommentUpdate
	DueDate *DueDateUpdate
	DueTime *DueTimeUpdate
	Repeat  *domain.RepeatKind
}

// RuleService orchestrates the task rule use cases (ADR 0051): create, read,
// partial update and delete. Every mutation runs through the shared
// runMutation conveyor, which owns the ordering invariants structurally: the
// role gate, the property row lock, the owner's today, the change step, its
// audit entry in the same transaction and the materialization tick when the
// step's verdict asks for it. Reads never tick and never write.
type RuleService struct {
	txStoreFactory
	policy    sharedpolicy.Policy
	calendar  OwnerCalendar
	writeGate gateFunc // Full Access and Owner: create/edit/delete (resolution #496).
}

// NewRuleService builds the rule use cases over the shared transactional
// store factory, the owner calendar and the authorization policy (ADR 0028).
// A nil calendar is a wiring mistake; a mutation fails on first use rather
// than writing with a zero date. A nil policy keeps the historical
// owner-only behaviour.
func NewRuleService(factory txStoreFactory, calendar OwnerCalendar, policy sharedpolicy.Policy) *RuleService {
	return &RuleService{
		txStoreFactory: factory,
		policy:         policy,
		calendar:       calendar,
		writeGate:      newCapabilityGate(policy, sharedpolicy.CanEdit),
	}
}

// conveyor bundles this service's factory and calendar for the shared
// mutation conveyor.
func (s *RuleService) conveyor() mutationGates {
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar}
}

// readScope applies the read gate and returns the data owner whose scope the
// SQL reads filter by (ADR 0028).
func (s *RuleService) readScope(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	return resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
}

// CreateRule creates a task rule on the property and materializes its first
// tasks in the same transaction: the undated task of an undated rule, every
// occurrence due today, and the single future one. The anchor acts from the
// owner's today on: a backdated due date is ErrInvalidInput — «прошлое не
// его задача» (resolution #496). Full Access and Owner may create; a viewer
// gets ErrForbidden, a stranger a privacy-preserving ErrNotFound.
func (s *RuleService) CreateRule(
	ctx context.Context, actor, propertyID uuid.UUID, cmd CreateRuleCommand,
) (domain.TaskRule, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, today time.Time,
		) (mutationOutcome[domain.TaskRule], error) {
			draft := domain.TaskRule{
				OwnerID:    scope,
				PropertyID: propertyID,
				Title:      strings.TrimSpace(cmd.Title),
				Comment:    cmd.Comment,
				DueDate:    cmd.DueDate,
				DueTime:    cmd.DueTime,
				Repeat:     cmd.Repeat,
			}
			if err := validateRule(draft); err != nil {
				return mutationOutcome[domain.TaskRule]{}, err
			}
			if err := validateAnchor(draft.DueDate, today); err != nil {
				return mutationOutcome[domain.TaskRule]{}, err
			}
			id, err := uuid.NewV7()
			if err != nil {
				return mutationOutcome[domain.TaskRule]{}, fmt.Errorf("mint rule id: %w", err)
			}
			draft.ID = id
			if err := stores.rules.Create(ctx, draft); err != nil {
				return mutationOutcome[domain.TaskRule]{}, fmt.Errorf("create task rule: %w", err)
			}
			return mutationOutcome[domain.TaskRule]{
				Response:      draft,
				Audit:         auditdomain.ActionTaskRuleCreated,
				AuditEntityID: &draft.ID,
				Tick:          true,
				RereadRuleID:  &draft.ID,
			}, nil
		})
}

// GetRule returns one rule. Any actor with the view capability may read; a
// missing or foreign rule is ErrNotFound. Reads never tick.
func (s *RuleService) GetRule(
	ctx context.Context, actor, propertyID, ruleID uuid.UUID,
) (domain.TaskRule, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return domain.TaskRule{}, err
	}
	return s.rules.Get(ctx, ruleID, scope, propertyID)
}

// UpdateRule applies a diff-patch to the rule: omitted fields are left
// unchanged, the optional ones are tri-state. The change invalidates the
// not-yet-due uncompleted tasks (the undated and strictly future ones) and
// the verdict keeps Tick=true so the tick stands the single future again
// with fresh snapshots; already due and completed tasks keep their frozen
// snapshots (tasks/CONTEXT.md «Задача»). A newly set due date must be today
// or later — a backdated anchor is ErrInvalidInput. Full Access and Owner
// may edit.
func (s *RuleService) UpdateRule(
	ctx context.Context, actor, propertyID, ruleID uuid.UUID, cmd UpdateRuleCommand,
) (domain.TaskRule, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, ruleID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, _ uuid.UUID, rule domain.TaskRule, today time.Time,
		) (mutationOutcome[domain.TaskRule], error) {
			applyUpdate(&rule, cmd)
			if err := validateRule(rule); err != nil {
				return mutationOutcome[domain.TaskRule]{}, err
			}
			// Only a newly set anchor faces the no-backdating check: an
			// unchanged past anchor belongs to the already created tasks.
			if cmd.DueDate != nil && cmd.DueDate.Value != nil {
				if err := validateAnchor(rule.DueDate, today); err != nil {
					return mutationOutcome[domain.TaskRule]{}, err
				}
			}
			if err := stores.rules.Update(ctx, rule); err != nil {
				return mutationOutcome[domain.TaskRule]{}, fmt.Errorf("update task rule: %w", err)
			}
			if err := stores.rules.DeleteNotDueUncompleted(ctx, rule.ID, today); err != nil {
				return mutationOutcome[domain.TaskRule]{}, err
			}
			return mutationOutcome[domain.TaskRule]{
				Response:      rule,
				Audit:         auditdomain.ActionTaskRuleUpdated,
				AuditEntityID: &rule.ID,
				AuditCtx:      map[string]any{"fields": updatedFields(cmd)},
				Tick:          true,
				RereadRuleID:  &rule.ID,
			}, nil
		})
}

// DeleteRule removes the rule hard (resolution #496 — жёсткое удаление, без
// надгробий): every uncompleted task of the rule — planned, overdue,
// today's and the single future — is physically removed; the completed
// journal stays with rule_id set to NULL by the FK and its snapshots. The
// verdict states Tick=false explicitly: the rule is gone — the tick has
// nothing to materialize. Full Access and Owner may delete.
func (s *RuleService) DeleteRule(
	ctx context.Context, actor, propertyID, ruleID uuid.UUID,
) error {
	_, err := runMutation(s.conveyor(), ctx, actor, propertyID, ruleID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.TaskRule, _ time.Time,
		) (mutationOutcome[domain.TaskRule], error) {
			// The uncompleted first: they hold the rule FK, and the rule row's
			// DELETE would otherwise leave them rule-less via SET NULL.
			if err := stores.rules.DeleteUncompleted(ctx, rule.ID); err != nil {
				return mutationOutcome[domain.TaskRule]{}, err
			}
			if err := stores.rules.Delete(ctx, rule.ID, scope); err != nil {
				return mutationOutcome[domain.TaskRule]{}, fmt.Errorf("delete task rule: %w", err)
			}
			return mutationOutcome[domain.TaskRule]{
				Response:      rule, // The pre-delete state; nothing re-reads it.
				Audit:         auditdomain.ActionTaskRuleDeleted,
				AuditEntityID: &rule.ID,
			}, nil
		})
	return err
}

// validateRule is the single validator of the create/update contract
// invariants — the transport decodes and delegates here, so the rules cannot
// drift between layers. It enforces: a known repeat, a non-empty title
// within MaxTitleLength characters and a comment within MaxCommentLength
// (counted in runes: the limits are product copy limits, not byte limits),
// the time-requires-date pairing, and the non-once repeat requiring a dated
// anchor (an undated rule is a once rule — the schema constraints
// task_rules_time_requires_date and task_rules_repeat_needs_anchor mirror
// these at the storage layer).
func validateRule(rule domain.TaskRule) error {
	title := strings.TrimSpace(rule.Title)
	if title == "" || utf8.RuneCountInString(title) > MaxTitleLength {
		return ErrInvalidInput
	}
	if rule.Comment != nil && utf8.RuneCountInString(*rule.Comment) > MaxCommentLength {
		return ErrInvalidInput
	}
	if !domain.IsValidRepeat(rule.Repeat) {
		return ErrInvalidInput
	}
	if rule.DueTime != nil && rule.DueDate == nil {
		return ErrInvalidInput
	}
	if rule.Repeat != domain.RepeatOnce && rule.DueDate == nil {
		return ErrInvalidInput
	}
	return nil
}

// validateAnchor enforces the no-backdating rule (resolution #496): a rule's
// anchor is today or later in the data owner's calendar. Applied at creation
// and whenever the command sets a new date; an unchanged past anchor belongs
// to the already created tasks.
func validateAnchor(dueDate *time.Time, today time.Time) error {
	if dueDate != nil && dueDate.Before(today) {
		return ErrInvalidInput
	}
	return nil
}

// applyUpdate folds the command's diff into the rule; only provided fields
// change, each validated as at creation by the caller's validateRule pass.
func applyUpdate(rule *domain.TaskRule, cmd UpdateRuleCommand) {
	if cmd.Title != nil {
		rule.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Comment != nil {
		rule.Comment = cmd.Comment.Value
	}
	if cmd.DueDate != nil {
		rule.DueDate = cmd.DueDate.Value
	}
	if cmd.DueTime != nil {
		rule.DueTime = cmd.DueTime.Value
	}
	if cmd.Repeat != nil {
		rule.Repeat = *cmd.Repeat
	}
}

// updatedFields lists the field names a command changes; the audit context
// carries names only, never values (the title is user data).
func updatedFields(cmd UpdateRuleCommand) []string {
	fields := make([]string, 0, 5)
	if cmd.Title != nil {
		fields = append(fields, "title")
	}
	if cmd.Comment != nil {
		fields = append(fields, "comment")
	}
	if cmd.DueDate != nil {
		fields = append(fields, "due_date")
	}
	if cmd.DueTime != nil {
		fields = append(fields, "due_time")
	}
	if cmd.Repeat != nil {
		fields = append(fields, "repeat")
	}
	return fields
}
