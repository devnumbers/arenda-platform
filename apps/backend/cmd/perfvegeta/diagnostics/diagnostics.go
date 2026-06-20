// Package diagnostics gathers host, PostgreSQL, and backend pgx pool metrics
// used to classify saturation during a perfvegeta run.
package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/model"
)

// DetectHostLimits collects OS-level resource limits.
func DetectHostLimits(ctx context.Context, profile model.GeneratorProfile) model.HostLimits {
	limits := model.HostLimits{
		OS:   runtime.GOOS,
		CPUs: runtime.NumCPU(),
	}

	var errMessages []string
	var nofile syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &nofile); err == nil {
		limits.UlimitNoFile = rlimitToInt(nofile.Cur)
	} else {
		errMessages = append(errMessages, fmt.Sprintf("ulimit -n: %v", err))
	}

	if runtime.GOOS == "darwin" {
		if value, err := readSysctlInt(ctx, "kern.maxfilesperproc"); err == nil {
			limits.MaxFilesPerProc = value
		} else {
			errMessages = append(errMessages, fmt.Sprintf("kern.maxfilesperproc: %v", err))
		}
		if value, err := readSysctlInt(ctx, "net.inet.ip.portrange.first"); err == nil {
			limits.PortRangeFirst = value
		} else {
			errMessages = append(errMessages, fmt.Sprintf("net.inet.ip.portrange.first: %v", err))
		}
		if value, err := readSysctlInt(ctx, "net.inet.ip.portrange.last"); err == nil {
			limits.PortRangeLast = value
		} else {
			errMessages = append(errMessages, fmt.Sprintf("net.inet.ip.portrange.last: %v", err))
		}
		if value, err := readSysctlUint64(ctx, "hw.memsize"); err == nil {
			limits.MemoryBytes = value
		} else {
			errMessages = append(errMessages, fmt.Sprintf("hw.memsize: %v", err))
		}
	}

	if limits.PortRangeLast > limits.PortRangeFirst {
		limits.EphemeralPorts = limits.PortRangeLast - limits.PortRangeFirst + 1
	}

	warnings := make([]string, 0, 3)
	if limits.EphemeralPorts > 0 && profile.MaxConnections > int(float64(limits.EphemeralPorts)*0.80) {
		warnings = append(warnings, fmt.Sprintf("max_connections=%d is above 80%% of available ephemeral ports (%d)", profile.MaxConnections, limits.EphemeralPorts))
	}
	if limits.MaxFilesPerProc > 0 && profile.MaxConnections > limits.MaxFilesPerProc-512 {
		warnings = append(warnings, fmt.Sprintf("max_connections=%d leaves little FD headroom below kern.maxfilesperproc=%d", profile.MaxConnections, limits.MaxFilesPerProc))
	}
	if profile.Workers > profile.MaxWorkers {
		warnings = append(warnings, fmt.Sprintf("workers=%d exceeds max_workers=%d", profile.Workers, profile.MaxWorkers))
	}
	if profile.Connections > profile.MaxConnections {
		warnings = append(warnings, fmt.Sprintf("connections=%d exceeds max_connections=%d", profile.Connections, profile.MaxConnections))
	}

	limits.ConfigWarnings = warnings
	limits.DiagnosticsErrors = errMessages
	return limits
}

func readSysctlInt(ctx context.Context, name string) (int, error) {
	// #nosec G204 -- sysctl key is selected by this command for read-only host diagnostics.
	out, err := exec.CommandContext(ctx, "sysctl", "-n", name).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func readSysctlUint64(ctx context.Context, name string) (uint64, error) {
	// #nosec G204 -- sysctl key is selected by this command for read-only host diagnostics.
	out, err := exec.CommandContext(ctx, "sysctl", "-n", name).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
}

func rlimitToInt(value uint64) int {
	maxValue := uint64(^uint(0) >> 1)
	if value > maxValue {
		return int(maxValue)
	}
	// #nosec G115 -- value is clamped to the maximum int above.
	return int(value)
}

// PrintHostDiagnostics writes the generator profile and host limits to stdout.
func PrintHostDiagnostics(limits model.HostLimits, profile model.GeneratorProfile) {
	fmt.Printf("generator profile: start=%d/s max=%d/s step=%d duration=%s timeout=%s cooldown=%s confirmations=%d connections=%d max_connections=%d workers=%d max_workers=%d max_body=%d\n",
		profile.StartRate,
		profile.MaxRate,
		profile.RateStep,
		profile.DurationText,
		profile.TimeoutText,
		profile.CooldownText,
		profile.Confirmations,
		profile.Connections,
		profile.MaxConnections,
		profile.Workers,
		profile.MaxWorkers,
		profile.MaxBody,
	)
	fmt.Printf("host diagnostics: os=%s cpus=%d ulimit_nofile=%d maxfilesperproc=%d ephemeral_ports=%d\n",
		limits.OS,
		limits.CPUs,
		limits.UlimitNoFile,
		limits.MaxFilesPerProc,
		limits.EphemeralPorts,
	)
	for _, warning := range limits.ConfigWarnings {
		fmt.Printf("generator warning: %s\n", warning)
	}
	for _, err := range limits.DiagnosticsErrors {
		fmt.Printf("diagnostics warning: %s\n", err)
	}
}

// ResetPostgresStatementStats calls pg_stat_statements_reset if possible.
func ResetPostgresStatementStats(parent context.Context) string {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL())
	if err != nil {
		return err.Error()
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, "SELECT pg_stat_statements_reset()"); err != nil {
		return err.Error()
	}
	return ""
}

// CollectPostgresSignal gathers PostgreSQL connection and statement statistics.
func CollectPostgresSignal(parent context.Context, statementResetErr string) model.PostgresSignal {
	databaseURL := os.Getenv("DATABASE_URL")
	databaseURLSet := databaseURL != ""
	if databaseURL == "" {
		databaseURL = model.DefaultDatabaseURL
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return model.PostgresSignal{DatabaseURLSet: databaseURLSet, StatementStatsResetErr: statementResetErr, Error: err.Error(), Reason: "could not connect to PostgreSQL for diagnostics"}
	}
	defer pool.Close()

	var maxConnText string
	if err := pool.QueryRow(ctx, "SHOW max_connections").Scan(&maxConnText); err != nil {
		return model.PostgresSignal{Available: true, DatabaseURLSet: databaseURLSet, StatementStatsResetErr: statementResetErr, Error: err.Error(), Reason: "could not read PostgreSQL max_connections"}
	}
	maxConn, _ := strconv.Atoi(maxConnText)

	var total int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&total); err != nil {
		return model.PostgresSignal{Available: true, DatabaseURLSet: databaseURLSet, StatementStatsResetErr: statementResetErr, Error: err.Error(), Reason: "could not read PostgreSQL total connections"}
	}

	var active int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&active); err != nil {
		return model.PostgresSignal{Available: true, DatabaseURLSet: databaseURLSet, StatementStatsResetErr: statementResetErr, Error: err.Error(), Reason: "could not read PostgreSQL active connections"}
	}

	var waiting int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND wait_event_type IS NOT NULL AND wait_event_type <> 'Client'").Scan(&waiting); err != nil {
		return model.PostgresSignal{Available: true, DatabaseURLSet: databaseURLSet, StatementStatsResetErr: statementResetErr, Error: err.Error(), Reason: "could not read PostgreSQL wait events"}
	}

	signal := model.PostgresSignal{
		Available:              true,
		DatabaseURLSet:         databaseURLSet,
		TotalConnections:       total,
		ActiveConnections:      active,
		MaxConnections:         maxConn,
		WaitingBackends:        waiting,
		StatementStatsResetErr: statementResetErr,
	}
	topStatements, err := collectTopPostgresStatements(ctx, pool)
	if err != nil {
		signal.StatementStatsError = err.Error()
	} else {
		signal.StatementStatsAvailable = true
		signal.TopStatements = topStatements
	}
	if maxConn > 0 && total >= int(float64(maxConn)*0.90) && active >= int(float64(maxConn)*0.50) {
		signal.Saturated = true
		signal.Reason = fmt.Sprintf("PostgreSQL total connections %d are near max_connections %d with %d active backends", total, maxConn, active)
	}
	if waiting > 0 {
		signal.Saturated = true
		if signal.Reason != "" {
			signal.Reason += "; "
		}
		signal.Reason += fmt.Sprintf("PostgreSQL has %d backends waiting on wait events", waiting)
	}
	if signal.Reason == "" {
		signal.Reason = "PostgreSQL diagnostics did not show saturation"
	}
	return signal
}

func collectTopPostgresStatements(ctx context.Context, pool *pgxpool.Pool) ([]model.PostgresStatementStat, error) {
	rows, err := pool.Query(ctx, `
SELECT
	queryid::text,
	calls,
	total_exec_time,
	mean_exec_time,
	max_exec_time,
	rows,
	shared_blks_hit,
	shared_blks_read,
	left(regexp_replace(query, '\s+', ' ', 'g'), 160) AS query
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY total_exec_time DESC
LIMIT 5
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []model.PostgresStatementStat
	for rows.Next() {
		var stat model.PostgresStatementStat
		if err := rows.Scan(
			&stat.QueryID,
			&stat.Calls,
			&stat.TotalExecTimeMS,
			&stat.MeanExecTimeMS,
			&stat.MaxExecTimeMS,
			&stat.Rows,
			&stat.SharedBlksHit,
			&stat.SharedBlksRead,
			&stat.Query,
		); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

// CollectBackendPoolSnapshot queries the backend's internal diagnostics endpoint.
func CollectBackendPoolSnapshot(parent context.Context, baseURL string, client *http.Client) model.DBPoolSnapshot {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/internal/perf/db-pool", nil)
	if err != nil {
		return model.DBPoolSnapshot{Error: err.Error()}
	}

	resp, err := client.Do(req)
	if err != nil {
		return model.DBPoolSnapshot{Error: err.Error()}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return model.DBPoolSnapshot{Error: fmt.Sprintf("backend pool diagnostics returned HTTP %d", resp.StatusCode)}
	}

	var snapshot model.DBPoolSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return model.DBPoolSnapshot{Error: err.Error()}
	}
	return snapshot
}

// BuildBackendPoolSignal compares before/after snapshots and returns a signal.
func BuildBackendPoolSignal(before, after model.DBPoolSnapshot) model.BackendPoolSignal {
	signal := model.BackendPoolSignal{
		Before: before,
		After:  after,
	}

	var errMessages []string
	if before.Error != "" {
		errMessages = append(errMessages, "before: "+before.Error)
	}
	if after.Error != "" {
		errMessages = append(errMessages, "after: "+after.Error)
	}
	if len(errMessages) > 0 {
		signal.Error = strings.Join(errMessages, "; ")
		signal.Reason = "backend pool diagnostics unavailable"
		return signal
	}
	if !before.Available || !after.Available {
		signal.Reason = "backend pool diagnostics unavailable"
		return signal
	}

	signal.Available = true
	signal.Delta = model.DBPoolDelta{
		AcquireCount:           nonNegativeInt64(after.AcquireCount - before.AcquireCount),
		AcquireDurationMS:      nonNegativeFloat(after.AcquireDurationMS - before.AcquireDurationMS),
		CanceledAcquireCount:   nonNegativeInt64(after.CanceledAcquireCount - before.CanceledAcquireCount),
		EmptyAcquireCount:      nonNegativeInt64(after.EmptyAcquireCount - before.EmptyAcquireCount),
		EmptyAcquireWaitTimeMS: nonNegativeFloat(after.EmptyAcquireWaitTimeMS - before.EmptyAcquireWaitTimeMS),
		NewConnsCount:          nonNegativeInt64(after.NewConnsCount - before.NewConnsCount),
	}

	var reasons []string
	if after.MaxConns > 0 && after.AcquiredConns >= int32(float64(after.MaxConns)*0.90) {
		reasons = append(reasons, fmt.Sprintf("backend pgx pool acquired %d of %d connections", after.AcquiredConns, after.MaxConns))
	}
	if signal.Delta.EmptyAcquireCount > 0 {
		reasons = append(reasons, fmt.Sprintf("backend pgx pool had %d empty acquires during probe", signal.Delta.EmptyAcquireCount))
	}
	if signal.Delta.CanceledAcquireCount > 0 {
		reasons = append(reasons, fmt.Sprintf("backend pgx pool had %d canceled acquires during probe", signal.Delta.CanceledAcquireCount))
	}

	if len(reasons) > 0 {
		signal.Saturated = true
		signal.Reason = strings.Join(reasons, "; ")
		return signal
	}

	signal.Reason = "backend pool diagnostics did not show saturation"
	return signal
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func databaseURL() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	return model.DefaultDatabaseURL
}
