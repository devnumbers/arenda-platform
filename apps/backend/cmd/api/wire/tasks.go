package wire

import (
	"fmt"

	taskspg "github.com/nambers/arenda-planform/apps/backend/internal/tasks/adapters/postgres"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
)

// Tasks holds the tasks module's services wired by WireTasks: the task rule
// use cases, the task use cases (listings, completion toggle, journal
// clear) and the materialization tick service — the worker's single door to
// the tick (the scheduler registers it as its own hourly loop, ADR 0051).
type Tasks struct {
	RuleService *tasksapp.RuleService
	TaskService *tasksapp.TaskService
	TickService *tasksapp.TickService
}

// WireTasks constructs the tasks context (ADR 0051): the tick store, the
// rule, task and property stores, the owner clock (both the calendar and the
// read-side moment ports) and the tick zone directory (ADR 0048), the single
// canonical txStoreFactory shared by every tasks service (ADR 0033
// γ-factory), the heartbeat metrics of the tick sweep and the tick service
// the hourly worker drives. The policy comes from the access module — tasks
// is wired after it, so the membership-aware policy is already resolved.
func WireTasks(p platformDeps) (*Tasks, error) {
	tickStore := taskspg.NewTickStore(p.DB)
	ruleStore := taskspg.NewRuleStore(p.DB)
	taskStore := taskspg.NewTaskStore(p.DB)
	propertyStore := taskspg.NewPropertyStore(p.DB)
	ownerClock := taskspg.NewOwnerClock(p.DB, p.Clock)
	zones := taskspg.NewTickZoneDirectory(p.DB)

	factory := tasksapp.NewTxStoreFactory(
		tickStore,
		ruleStore,
		taskStore,
		propertyStore,
		p.AuditRecorder,
		p.UoW,
	)

	metrics, err := tasksapp.NewMetrics()
	if err != nil {
		return nil, fmt.Errorf("tasks tick metrics: %w", err)
	}

	return &Tasks{
		RuleService: tasksapp.NewRuleService(factory, ownerClock, p.Policy),
		TaskService: tasksapp.NewTaskService(factory, ownerClock, ownerClock, p.Policy),
		TickService: tasksapp.NewTickService(factory, zones, ownerClock, metrics),
	}, nil
}
