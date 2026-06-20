// Package vegeta invokes the Vegeta binary and translates its output into
// structured probe results.
package vegeta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/diagnostics"
	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/model"
)

// ResolvePath substitutes placeholder IDs into an endpoint path.
func ResolvePath(path string, owner model.FixtureOwner, fx model.Fixtures) string {
	replacements := map[string]string{
		"{property_id}":            firstNonEmpty(owner.PropertyID, fx.PropertyID),
		"{lease_id}":               firstNonEmpty(owner.LeaseID, fx.LeaseID),
		"{operation_id}":           firstNonEmpty(owner.OperationID, fx.OperationID),
		"{recurring_operation_id}": firstNonEmpty(owner.RecurringOperationID, fx.RecurringOperationID),
		"{reminder_id}":            firstNonEmpty(owner.ReminderID, fx.ReminderID),
		"{contact_id}":             firstNonEmpty(owner.TenantContactID, fx.ContactID),
	}

	resolved := path
	for key, value := range replacements {
		resolved = strings.ReplaceAll(resolved, key, value)
	}
	return resolved
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// WriteTargets writes the Vegeta HTTP target file for an endpoint.
func WriteTargets(path string, cfg model.RunConfig, ep model.Endpoint) error {
	var b strings.Builder
	fx := cfg.Fixtures
	owners := append([]model.FixtureOwner(nil), fx.Owners...)
	rand.Shuffle(len(owners), func(i, j int) {
		owners[i], owners[j] = owners[j], owners[i]
	})

	for _, owner := range owners {
		url := cfg.BaseURL + ResolvePath(ep.Path, owner, fx)
		fmt.Fprintf(&b, "%s %s\n", ep.Method, url)
		fmt.Fprintf(&b, "Cookie: session_id=%s\n\n", owner.SessionToken)
	}

	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// Attack runs a single Vegeta attack against an endpoint at the given rate.
func Attack(ctx context.Context, cfg model.RunConfig, ep model.Endpoint, rate uint64, client *http.Client) model.ProbeResult {
	targetsFile := filepath.Join(cfg.ReportDir, fmt.Sprintf("%s_%d_targets.txt", ep.Name, rate))
	resultsFile := filepath.Join(cfg.ReportDir, fmt.Sprintf("%s_%d.bin", ep.Name, rate))

	if err := WriteTargets(targetsFile, cfg, ep); err != nil {
		return commandErrorProbe(cfg, ep, rate, fmt.Sprintf("write targets: %v", err), "")
	}
	defer func() {
		_ = os.Remove(targetsFile)
	}()

	statementResetErr := diagnostics.ResetPostgresStatementStats(ctx)
	poolBefore := diagnostics.CollectBackendPoolSnapshot(ctx, cfg.BaseURL, client)

	// #nosec G204 -- this local perf harness intentionally invokes the vegeta binary with generated local paths.
	attack := exec.CommandContext(ctx, "vegeta", "attack",
		"-targets", targetsFile,
		"-format", "http",
		"-rate", fmt.Sprintf("%d/s", rate),
		"-duration", cfg.Generator.Duration.String(),
		"-timeout", cfg.Generator.Timeout.String(),
		"-connections", strconv.Itoa(cfg.Generator.Connections),
		"-max-connections", strconv.Itoa(cfg.Generator.MaxConnections),
		"-workers", strconv.Itoa(cfg.Generator.Workers),
		"-max-workers", strconv.Itoa(cfg.Generator.MaxWorkers),
		"-max-body", strconv.Itoa(cfg.Generator.MaxBody),
		"-output", resultsFile,
	)
	var attackStderr bytes.Buffer
	attack.Stdout = io.Discard
	attack.Stderr = io.MultiWriter(os.Stderr, &attackStderr)

	runErr := attack.Run()
	poolAfter := diagnostics.CollectBackendPoolSnapshot(ctx, cfg.BaseURL, client)
	poolSignal := diagnostics.BuildBackendPoolSignal(poolBefore, poolAfter)
	if runErr != nil {
		probe := commandErrorProbe(cfg, ep, rate, fmt.Sprintf("vegeta attack: %v", runErr), attackStderr.String())
		probe.BackendPool = poolSignal
		return probe
	}

	// #nosec G204 -- this local perf harness intentionally invokes the vegeta binary with a generated local result file.
	reportCmd := exec.CommandContext(ctx, "vegeta", "report", "-type", "json", resultsFile)
	var reportOut bytes.Buffer
	var reportStderr bytes.Buffer
	reportCmd.Stdout = &reportOut
	reportCmd.Stderr = io.MultiWriter(os.Stderr, &reportStderr)
	reportErr := reportCmd.Run()
	if reportErr != nil {
		probe := commandErrorProbe(cfg, ep, rate, fmt.Sprintf("vegeta report: %v", reportErr), attackStderr.String()+reportStderr.String())
		probe.BackendPool = poolSignal
		return probe
	}

	var rep model.Report
	if err := json.Unmarshal(reportOut.Bytes(), &rep); err != nil {
		probe := commandErrorProbe(cfg, ep, rate, fmt.Sprintf("decode vegeta report: %v", err), attackStderr.String()+reportStderr.String())
		probe.BackendPool = poolSignal
		return probe
	}

	pg := diagnostics.CollectPostgresSignal(ctx, statementResetErr)
	probe := reportToProbe(cfg, ep, rate, rep, attackStderr.String()+reportStderr.String(), pg, poolSignal)
	fmt.Printf(
		"%s %-34s rate=%6d/s vegeta=%8.1f/s throughput=%8.1f/s success=%6.2f%% p95=%7.1fms wait=%5.2fs pass=%t pool_warning=%t bottleneck=%s\n",
		cfg.Mode,
		ep.Name,
		probe.RequestedRate,
		probe.VegetaRate,
		probe.Throughput,
		probe.Success*100,
		probe.P95MS,
		probe.WaitSec,
		probe.Pass,
		probe.PoolWarning,
		probe.Bottleneck,
	)
	return probe
}

func reportToProbe(cfg model.RunConfig, ep model.Endpoint, requestedRate uint64, rep model.Report, stderr string, pg model.PostgresSignal, backendPool model.BackendPoolSignal) model.ProbeResult {
	errorInputs := append([]string{}, rep.Errors...)
	if stderr != "" {
		errorInputs = append(errorInputs, stderr)
	}

	classes := classifyErrorClasses(errorInputs, rep.StatusCode)
	throughputRatio := 0.0
	if requestedRate > 0 {
		throughputRatio = rep.Throughput / float64(requestedRate)
	}

	p95MS := nanosToMillis(rep.Latencies.P95)
	pass := rep.Requests > 0 &&
		rep.Success == 1 &&
		hasOnlyExpectedStatuses(rep.StatusCode) &&
		len(rep.Errors) == 0 &&
		!hasGeneratorClass(classes) &&
		p95MS < float64(model.TargetP95/time.Millisecond)

	bottleneck, cause := classifyBottleneck(pass, classes, rep, pg, backendPool, requestedRate, throughputRatio)
	poolWarning, poolWarningCause := poolWarningSignal(pass, backendPool)

	status := "probe_failed"
	if pass {
		status = "probe_passed"
	}
	if bottleneck == "generator" {
		status = "invalid_generator_limit"
	}

	return model.ProbeResult{
		Endpoint:         ep.Name,
		Mode:             cfg.Mode,
		Status:           status,
		RequestedRate:    requestedRate,
		VegetaRate:       rep.Rate,
		Throughput:       rep.Throughput,
		ThroughputRatio:  throughputRatio,
		Success:          rep.Success,
		MeanMS:           nanosToMillis(rep.Latencies.Mean),
		P95MS:            p95MS,
		P99MS:            nanosToMillis(rep.Latencies.P99),
		MaxMS:            nanosToMillis(rep.Latencies.Max),
		WaitSec:          nanosToSeconds(rep.Wait),
		DurationSec:      nanosToSeconds(rep.Duration),
		Requests:         rep.Requests,
		StatusCodes:      rep.StatusCode,
		ErrorCount:       len(rep.Errors),
		ErrorClasses:     classes,
		Errors:           rep.Errors,
		Stderr:           strings.TrimSpace(stderr),
		Pass:             pass,
		PoolWarning:      poolWarning,
		PoolWarningCause: poolWarningCause,
		Bottleneck:       bottleneck,
		BottleneckCause:  cause,
		Generator:        cfg.Generator,
		Host:             cfg.Host,
		Postgres:         pg,
		BackendPool:      backendPool,
	}
}

func commandErrorProbe(cfg model.RunConfig, ep model.Endpoint, requestedRate uint64, errText string, stderr string) model.ProbeResult {
	errorInputs := []string{errText, stderr}
	classes := classifyErrorClasses(errorInputs, nil)
	bottleneck, cause := classifyBottleneck(false, classes, model.Report{}, model.PostgresSignal{}, model.BackendPoolSignal{}, requestedRate, 0)
	if bottleneck == "none" {
		bottleneck = "unknown"
		cause = errText
	}

	status := "probe_failed"
	if bottleneck == "generator" {
		status = "invalid_generator_limit"
	}

	return model.ProbeResult{
		Endpoint:        ep.Name,
		Mode:            cfg.Mode,
		Status:          status,
		RequestedRate:   requestedRate,
		StatusCodes:     map[string]int{},
		ErrorCount:      1,
		ErrorClasses:    classes,
		Errors:          []string{errText},
		Stderr:          strings.TrimSpace(stderr),
		Pass:            false,
		Bottleneck:      bottleneck,
		BottleneckCause: cause,
		Generator:       cfg.Generator,
		Host:            cfg.Host,
	}
}

func poolWarningSignal(pass bool, backendPool model.BackendPoolSignal) (bool, string) {
	if !pass || !backendPool.Saturated {
		return false, ""
	}
	return true, backendPool.Reason
}

func classifyErrorClasses(errors []string, statusCodes map[string]int) []string {
	classes := map[string]struct{}{}
	for _, text := range errors {
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "can't assign requested address"),
			strings.Contains(lower, "cannot assign requested address"),
			strings.Contains(lower, "bind:"):
			classes["generator_bind"] = struct{}{}
		case strings.Contains(lower, "too many open files"):
			classes["generator_fd"] = struct{}{}
		case strings.Contains(lower, "connection allocation"),
			strings.Contains(lower, "no buffer space available"),
			strings.Contains(lower, "socket:"):
			classes["generator_connection"] = struct{}{}
		case strings.Contains(lower, "max-workers"),
			strings.Contains(lower, "worker limit"):
			classes["generator_workers"] = struct{}{}
		case strings.Contains(lower, "timeout"),
			strings.Contains(lower, "deadline exceeded"):
			classes["backend_timeout"] = struct{}{}
		case strings.Contains(lower, "remaining connection slots"),
			strings.Contains(lower, "too many connections"),
			strings.Contains(lower, "pgxpool"),
			strings.Contains(lower, "postgres"):
			classes["postgres_connection"] = struct{}{}
		case strings.TrimSpace(lower) != "":
			classes["unknown_error"] = struct{}{}
		}
	}

	for code := range statusCodes {
		if strings.HasPrefix(code, "5") {
			classes["http_5xx"] = struct{}{}
		}
		if strings.HasPrefix(code, "4") {
			classes["http_4xx"] = struct{}{}
		}
	}

	result := make([]string, 0, len(classes))
	for class := range classes {
		result = append(result, class)
	}
	sort.Strings(result)
	return result
}

func classifyBottleneck(pass bool, classes []string, rep model.Report, pg model.PostgresSignal, backendPool model.BackendPoolSignal, requestedRate uint64, throughputRatio float64) (string, string) {
	if pass {
		return "none", "probe passed SLO"
	}

	pgSignal, pgCause := postgresLimitSignal(pg, classes)
	poolSignal, poolCause := backendPoolLimitSignal(backendPool)
	if hasGeneratorClass(classes) {
		if pgSignal || poolSignal {
			causes := make([]string, 0, 2)
			if pgSignal {
				causes = append(causes, pgCause)
			}
			if poolSignal {
				causes = append(causes, poolCause)
			}
			return "generator", "vegeta or local TCP/FD limits reported generator errors; backend/PostgreSQL also reported: " + strings.Join(causes, "; ")
		}
		return "generator", "vegeta or local TCP/FD limits reported generator errors"
	}

	if pgSignal {
		return "postgres", pgCause
	}
	if poolSignal {
		return "postgres", poolCause
	}

	if rep.Requests > 0 && (rep.Success < 1 || rep.Latencies.P95 >= int64(model.TargetP95) || rep.Wait > int64(2*time.Second) || (requestedRate > 0 && throughputRatio > 0 && throughputRatio < 0.90)) {
		return "backend", "latency/errors increased without generator or PostgreSQL saturation signals"
	}

	return "unknown", "no decisive generator, backend, or PostgreSQL signal was found"
}

func postgresLimitSignal(pg model.PostgresSignal, classes []string) (bool, string) {
	if pg.Saturated {
		return true, pg.Reason
	}
	if hasClass(classes, "postgres_connection") {
		return true, "request errors include PostgreSQL connection-limit signals"
	}

	errText := strings.ToLower(pg.Error)
	switch {
	case strings.Contains(errText, "too many clients"),
		strings.Contains(errText, "too many connections"),
		strings.Contains(errText, "remaining connection slots"):
		return true, "PostgreSQL diagnostics reported connection-limit error: " + pg.Error
	default:
		return false, ""
	}
}

func backendPoolLimitSignal(pool model.BackendPoolSignal) (bool, string) {
	if pool.Saturated {
		return true, pool.Reason
	}
	return false, ""
}

func hasGeneratorClass(classes []string) bool {
	for _, class := range classes {
		if strings.HasPrefix(class, "generator_") {
			return true
		}
	}
	return false
}

func hasClass(classes []string, needle string) bool {
	for _, class := range classes {
		if class == needle {
			return true
		}
	}
	return false
}

func hasOnlyExpectedStatuses(statusCodes map[string]int) bool {
	if len(statusCodes) == 0 {
		return false
	}
	for code := range statusCodes {
		if code != "200" {
			return false
		}
	}
	return true
}

func nanosToMillis(value int64) float64 {
	return float64(value) / float64(time.Millisecond)
}

func nanosToSeconds(value int64) float64 {
	return float64(value) / float64(time.Second)
}
