// Package report writes the JSON, CSV, and console summaries for a perfvegeta run.
package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/model"
)

// WriteReports persists the JSON and CSV reports for the run.
func WriteReports(dir string, summaries []model.EndpointSummary) (string, string, error) {
	jsonPath := filepath.Join(dir, "summary.json")
	jsonData, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o600); err != nil {
		return "", "", err
	}

	csvPath := filepath.Join(dir, "probes.csv")
	// #nosec G304 -- report path is generated under the run directory created by this command.
	file, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Warn("close csv report", slog.Any("error", closeErr))
		}
	}()

	writer := csv.NewWriter(file)

	header := []string{
		"endpoint", "mode", "status", "probe_roles", "requested_rate", "vegeta_rate", "throughput", "throughput_ratio",
		"success", "p95_ms", "p99_ms", "max_ms", "wait_sec", "requests", "pass", "pool_warning", "pool_warning_cause", "bottleneck",
		"bottleneck_cause", "error_count", "error_classes", "errors", "postgres_total_connections",
		"postgres_active_connections", "postgres_max_connections", "postgres_waiting_backends",
		"postgres_statement_stats_available", "postgres_statement_stats_error", "postgres_top_statement_query_id",
		"postgres_top_statement_calls", "postgres_top_statement_total_exec_ms", "postgres_top_statement_mean_exec_ms",
		"backend_pool_saturated", "backend_pool_reason", "backend_pool_acquired", "backend_pool_idle",
		"backend_pool_total", "backend_pool_max", "backend_pool_delta_empty_acquire_count",
		"backend_pool_delta_empty_acquire_wait_ms", "backend_pool_delta_acquire_duration_ms",
		"backend_pool_delta_canceled_acquire_count",
	}
	if err := writer.Write(header); err != nil {
		return "", "", err
	}

	for _, summary := range summaries {
		for _, probe := range summary.Probes {
			row := []string{
				probe.Endpoint,
				probe.Mode,
				probe.Status,
				strings.Join(probeRoles(summary, probe), "|"),
				strconv.FormatUint(probe.RequestedRate, 10),
				formatFloat(probe.VegetaRate),
				formatFloat(probe.Throughput),
				formatFloat(probe.ThroughputRatio),
				formatFloat(probe.Success),
				formatFloat(probe.P95MS),
				formatFloat(probe.P99MS),
				formatFloat(probe.MaxMS),
				formatFloat(probe.WaitSec),
				strconv.Itoa(probe.Requests),
				strconv.FormatBool(probe.Pass),
				strconv.FormatBool(probe.PoolWarning),
				probe.PoolWarningCause,
				probe.Bottleneck,
				probe.BottleneckCause,
				strconv.Itoa(probe.ErrorCount),
				strings.Join(probe.ErrorClasses, "|"),
				strings.Join(probe.Errors, "|"),
				strconv.Itoa(probe.Postgres.TotalConnections),
				strconv.Itoa(probe.Postgres.ActiveConnections),
				strconv.Itoa(probe.Postgres.MaxConnections),
				strconv.Itoa(probe.Postgres.WaitingBackends),
				strconv.FormatBool(probe.Postgres.StatementStatsAvailable),
				firstNonEmpty(probe.Postgres.StatementStatsError, probe.Postgres.StatementStatsResetErr),
				topStatementField(probe.Postgres.TopStatements, func(stat model.PostgresStatementStat) string { return stat.QueryID }),
				topStatementField(probe.Postgres.TopStatements, func(stat model.PostgresStatementStat) string { return strconv.FormatInt(stat.Calls, 10) }),
				topStatementField(probe.Postgres.TopStatements, func(stat model.PostgresStatementStat) string { return formatFloat(stat.TotalExecTimeMS) }),
				topStatementField(probe.Postgres.TopStatements, func(stat model.PostgresStatementStat) string { return formatFloat(stat.MeanExecTimeMS) }),
				strconv.FormatBool(probe.BackendPool.Saturated),
				probe.BackendPool.Reason,
				strconv.Itoa(int(probe.BackendPool.After.AcquiredConns)),
				strconv.Itoa(int(probe.BackendPool.After.IdleConns)),
				strconv.Itoa(int(probe.BackendPool.After.TotalConns)),
				strconv.Itoa(int(probe.BackendPool.After.MaxConns)),
				strconv.FormatInt(probe.BackendPool.Delta.EmptyAcquireCount, 10),
				formatFloat(probe.BackendPool.Delta.EmptyAcquireWaitTimeMS),
				formatFloat(probe.BackendPool.Delta.AcquireDurationMS),
				strconv.FormatInt(probe.BackendPool.Delta.CanceledAcquireCount, 10),
			}
			if err := writer.Write(row); err != nil {
				return "", "", err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", "", err
	}

	return jsonPath, csvPath, nil
}

func probeRoles(summary model.EndpointSummary, probe model.ProbeResult) []string {
	var roles []string
	if sameProbe(summary.Baseline5000Probe, probe) {
		roles = append(roles, "baseline_5000_probe")
	}
	if sameProbe(summary.BestSLOPass, probe) {
		roles = append(roles, "best_slo_pass")
	}
	if sameProbe(summary.FirstSLOFailObserved, probe) {
		roles = append(roles, "first_slo_fail_observed")
	}
	if sameProbe(summary.FirstGeneratorFail, probe) {
		roles = append(roles, "first_generator_fail")
	}
	return roles
}

func topStatementField(stats []model.PostgresStatementStat, value func(model.PostgresStatementStat) string) string {
	if len(stats) == 0 {
		return ""
	}
	return value(stats[0])
}

func sameProbe(marker *model.ProbeResult, probe model.ProbeResult) bool {
	return marker != nil &&
		marker.Endpoint == probe.Endpoint &&
		marker.Mode == probe.Mode &&
		marker.RequestedRate == probe.RequestedRate &&
		marker.Status == probe.Status
}

// PrintSummaryTable writes the human-readable run summary to stdout.
func PrintSummaryTable(summaries []model.EndpointSummary) {
	fmt.Println()
	fmt.Printf("%-34s %-12s %-28s %10s %10s %10s %8s %8s %8s %8s %8s %-6s %-5s %-10s %s\n",
		"endpoint",
		"mode",
		"status",
		"req/s",
		"veg/s",
		"thr/s",
		"ratio",
		"success",
		"p95",
		"wait",
		"requests",
		"pass",
		"pool",
		"bottleneck",
		"errors",
	)
	for _, summary := range summaries {
		fmt.Printf("%-34s %-12s %-28s %10d %10.1f %10.1f %8.2f %7.2f%% %7.1f %7.2f %8d %-6t %-5t %-10s %s\n",
			summary.Endpoint,
			summary.Mode,
			summary.Status,
			summary.RequestedRate,
			summary.VegetaRate,
			summary.Throughput,
			summary.ThroughputRatio,
			summary.Success*100,
			summary.P95MS,
			summary.WaitSec,
			summary.Requests,
			summary.Pass,
			summary.PoolWarning,
			summary.Bottleneck,
			strings.Join(summary.Errors, " | "),
		)
		if summary.Mode == "breakdown" && summary.LastSuccessRate > 0 {
			fmt.Printf("  last_success_rate=%d/s first_broken_rate=%d/s\n", summary.LastSuccessRate, summary.FirstBrokenRate)
		}
		if summary.Mode == "breakdown" {
			fmt.Println("  breakdown_note: breakdown is a stress map; use sustainable for confirmed SLO capacity")
		}
		if summary.Mode == "sustainable" && summary.Baseline5000Probe != nil {
			fmt.Printf("  baseline_5000_probe pass=%t p95=%.1fms success=%.2f%% bottleneck=%s\n",
				summary.Baseline5000Probe.Pass,
				summary.Baseline5000Probe.P95MS,
				summary.Baseline5000Probe.Success*100,
				summary.Baseline5000Probe.Bottleneck,
			)
		}
		if summary.BestSLOPass != nil {
			fmt.Printf("  best_slo_pass=%d/s p95=%.1fms success=%.2f%%\n", summary.BestSLOPass.RequestedRate, summary.BestSLOPass.P95MS, summary.BestSLOPass.Success*100)
		}
		if summary.FirstSLOFailObserved != nil {
			fmt.Printf("  first_slo_fail_observed=%d/s p95=%.1fms bottleneck=%s\n", summary.FirstSLOFailObserved.RequestedRate, summary.FirstSLOFailObserved.P95MS, summary.FirstSLOFailObserved.Bottleneck)
		}
		if summary.FirstGeneratorFail != nil {
			fmt.Printf("  first_generator_fail=%d/s errors=%s\n", summary.FirstGeneratorFail.RequestedRate, strings.Join(summary.FirstGeneratorFail.ErrorClasses, "|"))
		}
		if summary.UnstableBoundary {
			fmt.Println("  unstable_boundary=true: a lower SLO failure was followed by a higher SLO pass")
		}
		if summary.PoolWarning {
			fmt.Printf("  pool_warning: %s\n", summary.PoolWarningCause)
		}
		if summary.ConfirmationAttempts > 0 {
			fmt.Printf("  confirmation=%d/%d", summary.ConfirmationPasses, summary.ConfirmationAttempts)
			if summary.ConfirmedRate > 0 {
				fmt.Printf(" confirmed_rate=%d/s", summary.ConfirmedRate)
			}
			fmt.Println()
		}
		if summary.BottleneckCause != "" {
			fmt.Printf("  cause: %s\n", summary.BottleneckCause)
		}
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
