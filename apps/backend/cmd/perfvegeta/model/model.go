// Package model contains shared types for the perfvegeta load-testing harness.
package model

import "time"

const (
	TargetP95 = 500 * time.Millisecond

	BaselineRate      uint64 = 5_000
	StartRate                = BaselineRate
	MaxRate           uint64 = 50_000
	RateStep          uint64 = 500
	Tolerance         uint64 = 100
	BenchmarkDuration        = 30 * time.Second
	RequestTimeout           = 10 * time.Second
	ProbeCooldown            = 5 * time.Second

	SustainableConfirmationAttempts = 3

	BreakdownSuccessThreshold = 0.90

	VegetaConnections    = 12_000
	VegetaMaxConnections = 12_000
	VegetaWorkers        = 2_000
	VegetaMaxWorkers     = 12_000
	VegetaMaxBody        = 0

	DefaultBaseURL = "http://127.0.0.1:8081"
	// #nosec G101 -- local perf diagnostics default, not a production secret.
	DefaultDatabaseURL = "postgres://arenda:arenda@localhost:5434/arenda?sslmode=disable"
	FixturesPath       = "perf/fixtures.json"
	ResultsDir         = "perf/results"
)

// Endpoint describes a read endpoint that can be targeted by Vegeta.
type Endpoint struct {
	Name   string
	Method string
	Path   string
}

// Fixtures holds the test data produced by perfseed.
type Fixtures struct {
	Tokens               []string       `json:"tokens"`
	OwnerIDs             []string       `json:"owner_ids"`
	PropertyID           string         `json:"property_id"`
	LeaseID              string         `json:"lease_id"`
	OperationID          string         `json:"operation_id"`
	RecurringOperationID string         `json:"recurring_operation_id"`
	ReminderID           string         `json:"reminder_id"`
	ContactID            string         `json:"contact_id"`
	Owners               []FixtureOwner `json:"owners"`
}

// FixtureOwner ties a token to the IDs owned by that user.
type FixtureOwner struct {
	Index                int    `json:"index"`
	ID                   string `json:"id"`
	Phone                string `json:"phone"`
	SessionToken         string `json:"sessionToken"`
	PropertyID           string `json:"propertyID"`
	LeaseID              string `json:"leaseID"`
	OperationID          string `json:"operationID"`
	RecurringOperationID string `json:"recurringOperationID"`
	ReminderID           string `json:"reminderID"`
	TenantContactID      string `json:"tenantContactID"`
}

// Normalize ensures both the legacy top-level arrays and the per-owner
// structures are usable from the loaded fixtures.
func (fx *Fixtures) Normalize() {
	if len(fx.Tokens) == 0 && len(fx.Owners) > 0 {
		fx.Tokens = make([]string, 0, len(fx.Owners))
		fx.OwnerIDs = make([]string, 0, len(fx.Owners))
		for _, owner := range fx.Owners {
			fx.Tokens = append(fx.Tokens, owner.SessionToken)
			fx.OwnerIDs = append(fx.OwnerIDs, owner.ID)
		}
	}

	if len(fx.Owners) == 0 {
		fx.Owners = make([]FixtureOwner, 0, len(fx.Tokens))
		for i, token := range fx.Tokens {
			ownerID := ""
			if i < len(fx.OwnerIDs) {
				ownerID = fx.OwnerIDs[i]
			}
			fx.Owners = append(fx.Owners, FixtureOwner{
				Index:                i,
				ID:                   ownerID,
				SessionToken:         token,
				PropertyID:           fx.PropertyID,
				LeaseID:              fx.LeaseID,
				OperationID:          fx.OperationID,
				RecurringOperationID: fx.RecurringOperationID,
				ReminderID:           fx.ReminderID,
				TenantContactID:      fx.ContactID,
			})
		}
	}

	if len(fx.Owners) == 0 {
		return
	}
	primary := fx.Owners[0]
	if fx.PropertyID == "" {
		fx.PropertyID = primary.PropertyID
	}
	if fx.LeaseID == "" {
		fx.LeaseID = primary.LeaseID
	}
	if fx.OperationID == "" {
		fx.OperationID = primary.OperationID
	}
	if fx.RecurringOperationID == "" {
		fx.RecurringOperationID = primary.RecurringOperationID
	}
	if fx.ReminderID == "" {
		fx.ReminderID = primary.ReminderID
	}
	if fx.ContactID == "" {
		fx.ContactID = primary.TenantContactID
	}
}

// GeneratorProfile captures the Vegeta attack configuration.
type GeneratorProfile struct {
	StartRate      uint64        `json:"start_rate"`
	MaxRate        uint64        `json:"max_rate"`
	RateStep       uint64        `json:"rate_step"`
	Tolerance      uint64        `json:"tolerance"`
	Duration       time.Duration `json:"-"`
	DurationText   string        `json:"duration"`
	Timeout        time.Duration `json:"-"`
	TimeoutText    string        `json:"timeout"`
	ProbeCooldown  time.Duration `json:"-"`
	CooldownText   string        `json:"probe_cooldown"`
	Confirmations  int           `json:"sustainable_confirmations"`
	Connections    int           `json:"connections"`
	MaxConnections int           `json:"max_connections"`
	Workers        int           `json:"workers"`
	MaxWorkers     int           `json:"max_workers"`
	MaxBody        int           `json:"max_body"`
}

// HostLimits captures OS-level resource limits relevant to Vegeta.
type HostLimits struct {
	OS                string   `json:"os"`
	CPUs              int      `json:"cpus"`
	UlimitNoFile      int      `json:"ulimit_nofile,omitempty"`
	MaxFilesPerProc   int      `json:"maxfilesperproc,omitempty"`
	PortRangeFirst    int      `json:"port_range_first,omitempty"`
	PortRangeLast     int      `json:"port_range_last,omitempty"`
	EphemeralPorts    int      `json:"ephemeral_ports,omitempty"`
	MemoryBytes       uint64   `json:"memory_bytes,omitempty"`
	ConfigWarnings    []string `json:"config_warnings,omitempty"`
	DiagnosticsErrors []string `json:"diagnostics_errors,omitempty"`
}

// PostgresSignal captures PostgreSQL connection and statement statistics.
type PostgresSignal struct {
	Available               bool                  `json:"available"`
	DatabaseURLSet          bool                  `json:"database_url_set"`
	TotalConnections        int                   `json:"total_connections,omitempty"`
	ActiveConnections       int                   `json:"active_connections,omitempty"`
	MaxConnections          int                   `json:"max_connections,omitempty"`
	WaitingBackends         int                   `json:"waiting_backends,omitempty"`
	StatementStatsAvailable bool                  `json:"statement_stats_available"`
	StatementStatsResetErr  string                `json:"statement_stats_reset_error,omitempty"`
	StatementStatsError     string                `json:"statement_stats_error,omitempty"`
	TopStatements           []PostgresStatementStat `json:"top_statements,omitempty"`
	Saturated               bool                  `json:"saturated"`
	Reason                  string                `json:"reason,omitempty"`
	Error                   string                `json:"error,omitempty"`
}

// PostgresStatementStat is a single entry from pg_stat_statements.
type PostgresStatementStat struct {
	QueryID         string  `json:"query_id"`
	Calls           int64   `json:"calls"`
	TotalExecTimeMS float64 `json:"total_exec_time_ms"`
	MeanExecTimeMS  float64 `json:"mean_exec_time_ms"`
	MaxExecTimeMS   float64 `json:"max_exec_time_ms"`
	Rows            int64   `json:"rows"`
	SharedBlksHit   int64   `json:"shared_blks_hit"`
	SharedBlksRead  int64   `json:"shared_blks_read"`
	Query           string  `json:"query"`
}

// BackendPoolSignal captures the backend pgx pool state before and after a probe.
type BackendPoolSignal struct {
	Available bool           `json:"available"`
	Before    DBPoolSnapshot `json:"before,omitempty"`
	After     DBPoolSnapshot `json:"after,omitempty"`
	Delta     DBPoolDelta    `json:"delta,omitempty"`
	Saturated bool           `json:"saturated"`
	Reason    string         `json:"reason,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// DBPoolSnapshot is the JSON shape exposed by /internal/perf/db-pool.
type DBPoolSnapshot struct {
	Available              bool    `json:"available"`
	AcquiredConns          int32   `json:"acquired_conns"`
	IdleConns              int32   `json:"idle_conns"`
	TotalConns             int32   `json:"total_conns"`
	ConstructingConns      int32   `json:"constructing_conns"`
	MaxConns               int32   `json:"max_conns"`
	AcquireCount           int64   `json:"acquire_count"`
	AcquireDurationMS      float64 `json:"acquire_duration_ms"`
	CanceledAcquireCount   int64   `json:"canceled_acquire_count"`
	EmptyAcquireCount      int64   `json:"empty_acquire_count"`
	EmptyAcquireWaitTimeMS float64 `json:"empty_acquire_wait_time_ms"`
	NewConnsCount          int64   `json:"new_conns_count"`
	Error                  string  `json:"error,omitempty"`
}

// DBPoolDelta is the difference between two DBPoolSnapshot values.
type DBPoolDelta struct {
	AcquireCount           int64   `json:"acquire_count"`
	AcquireDurationMS      float64 `json:"acquire_duration_ms"`
	CanceledAcquireCount   int64   `json:"canceled_acquire_count"`
	EmptyAcquireCount      int64   `json:"empty_acquire_count"`
	EmptyAcquireWaitTimeMS float64 `json:"empty_acquire_wait_time_ms"`
	NewConnsCount          int64   `json:"new_conns_count"`
}

// Report mirrors the JSON report produced by `vegeta report -type json`.
type Report struct {
	Latencies struct {
		Mean int64 `json:"mean"`
		P95  int64 `json:"95th"`
		P99  int64 `json:"99th"`
		Max  int64 `json:"max"`
	} `json:"latencies"`
	BytesIn struct {
		Total int64   `json:"total"`
		Mean  float64 `json:"mean"`
	} `json:"bytes_in"`
	BytesOut struct {
		Total int64   `json:"total"`
		Mean  float64 `json:"mean"`
	} `json:"bytes_out"`
	Earliest   string         `json:"earliest"`
	Latest     string         `json:"latest"`
	End        string         `json:"end"`
	Duration   int64          `json:"duration"`
	Wait       int64          `json:"wait"`
	Requests   int            `json:"requests"`
	Rate       float64        `json:"rate"`
	Throughput float64        `json:"throughput"`
	Success    float64        `json:"success"`
	StatusCode map[string]int `json:"status_codes"`
	Errors     []string       `json:"errors"`
	Raw        map[string]any `json:"-"`
}

// ProbeResult is the parsed outcome of a single Vegeta attack.
type ProbeResult struct {
	Endpoint         string            `json:"endpoint"`
	Mode             string            `json:"mode"`
	Status           string            `json:"status"`
	RequestedRate    uint64            `json:"requested_rate"`
	VegetaRate       float64           `json:"vegeta_rate"`
	Throughput       float64           `json:"throughput"`
	ThroughputRatio  float64           `json:"throughput_ratio"`
	Success          float64           `json:"success"`
	MeanMS           float64           `json:"mean_ms"`
	P95MS            float64           `json:"p95_ms"`
	P99MS            float64           `json:"p99_ms"`
	MaxMS            float64           `json:"max_ms"`
	WaitSec          float64           `json:"wait_sec"`
	DurationSec      float64           `json:"duration_sec"`
	Requests         int               `json:"requests"`
	StatusCodes      map[string]int    `json:"status_codes"`
	ErrorCount       int               `json:"error_count"`
	ErrorClasses     []string          `json:"error_classes"`
	Errors           []string          `json:"errors,omitempty"`
	Stderr           string            `json:"stderr,omitempty"`
	Pass             bool              `json:"pass"`
	PoolWarning      bool              `json:"pool_warning"`
	PoolWarningCause string            `json:"pool_warning_cause,omitempty"`
	Bottleneck       string            `json:"bottleneck"`
	BottleneckCause  string            `json:"bottleneck_cause"`
	Generator        GeneratorProfile  `json:"generator"`
	Host             HostLimits        `json:"host"`
	Postgres         PostgresSignal    `json:"postgres"`
	BackendPool      BackendPoolSignal `json:"backend_pool"`
}

// EndpointSummary aggregates all probes for a single endpoint.
type EndpointSummary struct {
	Endpoint             string           `json:"endpoint"`
	Mode                 string           `json:"mode"`
	Status               string           `json:"status"`
	RequestedRate        uint64           `json:"requested_rate"`
	VegetaRate           float64          `json:"vegeta_rate"`
	Throughput           float64          `json:"throughput"`
	ThroughputRatio      float64          `json:"throughput_ratio"`
	Success              float64          `json:"success"`
	P95MS                float64          `json:"p95_ms"`
	P99MS                float64          `json:"p99_ms"`
	MaxMS                float64          `json:"max_ms"`
	WaitSec              float64          `json:"wait_sec"`
	Requests             int              `json:"requests"`
	Pass                 bool             `json:"pass"`
	Bottleneck           string           `json:"bottleneck"`
	BottleneckCause      string           `json:"bottleneck_cause"`
	Errors               []string         `json:"errors,omitempty"`
	LastSuccessRate      uint64           `json:"last_success_rate,omitempty"`
	LastGoodRate         uint64           `json:"last_good_rate,omitempty"`
	FirstBrokenRate      uint64           `json:"first_broken_rate,omitempty"`
	PoolWarning          bool             `json:"pool_warning"`
	PoolWarningCause     string           `json:"pool_warning_cause,omitempty"`
	UnstableBoundary     bool             `json:"unstable_boundary"`
	ConfirmationAttempts int              `json:"confirmation_attempts,omitempty"`
	ConfirmationPasses   int              `json:"confirmation_passes,omitempty"`
	ConfirmedRate        uint64           `json:"confirmed_rate,omitempty"`
	Generator            GeneratorProfile `json:"generator"`
	Host                 HostLimits       `json:"host"`
	BestSLOPass          *ProbeResult     `json:"best_slo_pass,omitempty"`
	BestPass             *ProbeResult     `json:"best_pass,omitempty"`
	Baseline5000Probe    *ProbeResult     `json:"baseline_5000_probe,omitempty"`
	FirstSLOFailObserved *ProbeResult     `json:"first_slo_fail_observed,omitempty"`
	FirstSLOFail         *ProbeResult     `json:"first_slo_fail,omitempty"`
	FirstGeneratorFail   *ProbeResult     `json:"first_generator_fail,omitempty"`
	Probes               []ProbeResult    `json:"probes"`
}

// RunConfig carries the top-level configuration for a perfvegeta run.
type RunConfig struct {
	Mode      string
	Endpoint  string
	BaseURL   string
	Fixtures  Fixtures
	Generator GeneratorProfile
	Host      HostLimits
	ReportDir string
}

// Endpoints is the list of read endpoints exercised by the harness.
var Endpoints = []Endpoint{
	{Name: "get_me", Method: "GET", Path: "/me"},
	{Name: "list_properties", Method: "GET", Path: "/properties"},
	{Name: "get_property", Method: "GET", Path: "/properties/{property_id}"},
	{Name: "list_operations", Method: "GET", Path: "/properties/{property_id}/operations"},
	{Name: "get_operation", Method: "GET", Path: "/operations/{operation_id}"},
	{Name: "list_recurring_operations", Method: "GET", Path: "/properties/{property_id}/recurring-operations"},
	{Name: "get_recurring_operation", Method: "GET", Path: "/recurring-operations/{recurring_operation_id}"},
	{Name: "list_reminders", Method: "GET", Path: "/reminders"},
	{Name: "list_lease_reminders", Method: "GET", Path: "/leases/{lease_id}/reminders"},
	{Name: "list_operation_reminders", Method: "GET", Path: "/properties/{property_id}/operations/{operation_id}/reminders"},
	{Name: "list_recurring_operation_reminders", Method: "GET", Path: "/properties/{property_id}/recurring-operations/{recurring_operation_id}/reminders"},
	{Name: "list_contacts", Method: "GET", Path: "/tenant-contacts"},
	{Name: "get_contact", Method: "GET", Path: "/tenant-contacts/{contact_id}"},
	{Name: "get_subscription", Method: "GET", Path: "/subscription"},
	{Name: "list_payment_methods", Method: "GET", Path: "/subscription/payment-methods"},
	{Name: "list_subscription_payments", Method: "GET", Path: "/subscription/payments"},
	{Name: "list_tariffs", Method: "GET", Path: "/tariffs"},
}
