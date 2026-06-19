package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	p95ThresholdMs     = 500.0
	errorRateThreshold = 0.01
	k6Image            = "grafana/k6:0.52.0"
)

var endpointManifest = []string{
	"get_me",
	"get_properties",
	"get_property",
	"list_operations",
	"get_operation",
	"list_recurring_operations",
	"get_recurring_operation",
	"list_leases",
	"get_lease",
	"list_tenant_contacts",
	"get_tenant_contact",
	"get_operation_reminders",
	"get_recurring_operation_reminders",
	"get_lease_reminders",
	"list_reminders",
	"get_tariffs",
	"get_subscription",
	"list_subscription_payments",
	"list_payment_methods",
	"auth_send_code",
	"auth_verify_code",
	"auth_logout",
	"post_property",
	"patch_property",
	"archive_property",
	"unarchive_property",
	"create_operation",
	"update_operation",
	"delete_operation",
	"create_recurring_operation",
	"update_recurring_operation",
	"pause_recurring_operation",
	"resume_recurring_operation",
	"create_lease",
	"update_lease",
	"complete_lease",
	"create_tenant_contact",
	"update_tenant_contact",
	"create_operation_reminder",
	"create_recurring_operation_reminder",
	"update_reminder",
	"delete_reminder",
	"patch_subscription_auto_renew",
	"cancel_subscription",
	"change_subscription",
	"add_payment_method",
	"delete_payment_method",
	"activate_payment_method",
	"confirm_fake_subscription_payment",
}

type config struct {
	endpoint   string
	suite      bool
	startRate  int
	maxRate    int
	maxIters   int
	duration   time.Duration
	apiBaseURL string
}

type endpointResult struct {
	Endpoint  string  `json:"endpoint"`
	MaxRPS    int     `json:"max_rps"`
	P95MS     float64 `json:"p95_ms"`
	ErrorRate float64 `json:"error_rate"`
	Passed    bool    `json:"passed"`
	Error     string  `json:"error,omitempty"`
}

type suiteResults struct {
	RunID      string           `json:"run_id"`
	Timestamp  string           `json:"timestamp"`
	APIBaseURL string           `json:"api_base_url"`
	Duration   string           `json:"duration"`
	StartRate  int              `json:"start_rate"`
	MaxRate    int              `json:"max_rate"`
	Endpoints  []endpointResult `json:"endpoints"`
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := parseConfig()
	if err != nil {
		logger.ErrorContext(ctx, "invalid configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	backendDir, err := os.Getwd()
	if err != nil {
		logger.ErrorContext(ctx, "get working directory", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := preflight(ctx, logger, cfg, backendDir); err != nil {
		logger.ErrorContext(ctx, "preflight check failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if cfg.suite {
		if err := runSuite(ctx, logger, cfg, backendDir); err != nil {
			logger.ErrorContext(ctx, "suite failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}

	maxRPS, p95, errorRate, passed, err := benchmarkEndpoint(ctx, logger, cfg.endpoint, cfg, backendDir)
	if err != nil {
		logger.ErrorContext(ctx, "benchmark failed", slog.String("endpoint", cfg.endpoint), slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.InfoContext(ctx, "benchmark complete",
		slog.String("endpoint", cfg.endpoint),
		slog.Int("max_rps", maxRPS),
		slog.Float64("p95_ms", p95),
		slog.Float64("error_rate", errorRate),
		slog.Bool("passed", passed),
	)

	if !passed {
		os.Exit(1)
	}
}

func parseConfig() (config, error) {
	var cfg config
	var err error

	cfg.startRate, err = envInt("START_RATE", 10)
	if err != nil {
		return cfg, err
	}

	cfg.maxRate, err = envInt("MAX_RATE", 1000)
	if err != nil {
		return cfg, err
	}

	cfg.maxIters, err = envInt("MAX_ITERS", 10)
	if err != nil {
		return cfg, err
	}

	cfg.duration, err = envDuration("DURATION", 15*time.Second)
	if err != nil {
		return cfg, err
	}

	cfg.apiBaseURL = envString("API_BASE_URL", "http://localhost:8081")

	endpoint := flag.String("endpoint", "", "single endpoint key to benchmark")
	suite := flag.Bool("suite", false, "run full endpoint suite")
	startRate := flag.Int("start-rate", cfg.startRate, "initial RPS for binary search")
	maxRate := flag.Int("max-rate", cfg.maxRate, "maximum RPS for binary search")
	maxIters := flag.Int("max-iters", cfg.maxIters, "maximum binary-search iterations")
	duration := flag.Duration("duration", cfg.duration, "duration of each k6 run")
	apiBaseURL := flag.String("api-base-url", cfg.apiBaseURL, "base URL of the API under test")
	flag.Parse()

	cfg.endpoint = *endpoint
	cfg.suite = *suite
	cfg.startRate = *startRate
	cfg.maxRate = *maxRate
	cfg.maxIters = *maxIters
	cfg.duration = *duration
	cfg.apiBaseURL = *apiBaseURL

	if cfg.endpoint != "" && cfg.suite {
		return cfg, fmt.Errorf("specify either -endpoint or -suite, not both")
	}
	if cfg.endpoint == "" && !cfg.suite {
		return cfg, fmt.Errorf("specify either -endpoint or -suite")
	}
	if cfg.startRate <= 0 || cfg.maxRate <= 0 || cfg.maxIters <= 0 {
		return cfg, fmt.Errorf("start-rate, max-rate and max-iters must be positive")
	}
	if cfg.startRate > cfg.maxRate {
		return cfg, fmt.Errorf("start-rate (%d) cannot exceed max-rate (%d)", cfg.startRate, cfg.maxRate)
	}

	return cfg, nil
}

func envString(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid %s: %s", key, v)
	}
	return n, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", key, v)
	}
	return d, nil
}

func preflight(ctx context.Context, logger *slog.Logger, cfg config, backendDir string) error {
	rootDir := filepath.Dir(backendDir)
	composeFile := filepath.Join(rootDir, "docker-compose.perf.yml")

	logger.InfoContext(ctx, "checking perf postgres container")
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", "arenda-perf", "-f", composeFile, "ps", "-q") //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker compose ps failed: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("perf postgres container is not running")
	}

	logger.InfoContext(ctx, "checking backend reachability", slog.String("url", cfg.apiBaseURL+"/me"))
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.apiBaseURL+"/me", nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("backend unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("backend /me returned status %d", resp.StatusCode)
	}

	return nil
}

func benchmarkEndpoint(ctx context.Context, logger *slog.Logger, endpoint string, cfg config, backendDir string) (int, float64, float64, bool, error) {
	logger.InfoContext(ctx, "re-seeding database", slog.String("endpoint", endpoint))
	if err := reseed(ctx, endpoint, backendDir); err != nil {
		return 0, 0, 0, false, fmt.Errorf("reseed endpoint %s: %w", endpoint, err)
	}

	low, high := cfg.startRate, cfg.maxRate
	best := 0
	var bestP95, bestErrorRate float64

	runOnce := func(rate int) (float64, float64, bool, error) {
		return runK6(ctx, endpoint, rate, cfg.duration, cfg.apiBaseURL, backendDir)
	}

	if low == high {
		p95, errorRate, passed, err := runOnce(low)
		if err != nil {
			return 0, 0, 0, false, err
		}
		if passed {
			best = low
			bestP95, bestErrorRate = p95, errorRate
		}
		return best, bestP95, bestErrorRate, passed, nil
	}

	for i := 0; i < cfg.maxIters; i++ {
		if high-low < 1 {
			break
		}
		// Use ceiling mid so the upper bound is always tested.
		mid := low + (high-low+1)/2
		p95, errorRate, passed, err := runOnce(mid)
		if err != nil {
			return 0, 0, 0, false, err
		}

		logger.InfoContext(ctx, "benchmark iteration",
			slog.String("endpoint", endpoint),
			slog.Int("rate", mid),
			slog.Float64("p95_ms", p95),
			slog.Float64("error_rate", errorRate),
			slog.Bool("passed", passed),
		)

		if passed {
			best = mid
			bestP95, bestErrorRate = p95, errorRate
			low = mid
		} else {
			high = mid - 1
		}
	}

	return best, bestP95, bestErrorRate, best > 0, nil
}

func reseed(ctx context.Context, endpoint string, backendDir string) error {
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/perfseed", "-endpoint="+endpoint) //nolint:gosec
	cmd.Dir = backendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runK6(ctx context.Context, endpoint string, rate int, duration time.Duration, apiBaseURL, backendDir string) (float64, float64, bool, error) {
	outDir, err := os.MkdirTemp("", "perfmaxrps-"+endpoint+"-*")
	if err != nil {
		return 0, 0, false, fmt.Errorf("create output dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(outDir) }()

	scriptsDir := filepath.Join(backendDir, "perf", "scripts")
	perfDir := filepath.Join(backendDir, "perf")

	args := []string{
		"run", "--rm",
		"-v", scriptsDir + ":/scripts:ro",
		"-v", perfDir + ":/perf:ro",
		"-v", outDir + ":/output",
		"-e", "ENDPOINT=" + endpoint,
		"-e", "RATE=" + strconv.Itoa(rate),
		"-e", "DURATION=" + duration.String(),
		"-e", "API_BASE_URL=" + apiBaseURL,
		"-e", "K6_OUTPUT_DIR=/output",
		k6Image,
		"run", "/scripts/endpoint_benchmark.js",
	}

	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec
	cmd.Dir = backendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()

	summaryPath := filepath.Join(outDir, "summary.json")
	data, readErr := os.ReadFile(summaryPath) //nolint:gosec
	if readErr != nil {
		if runErr != nil {
			return 0, 0, false, fmt.Errorf("k6 run failed and no summary was written: %w (read summary: %w)", runErr, readErr)
		}
		return 0, 0, false, fmt.Errorf("read summary.json: %w", readErr)
	}

	p95, errorRate, err := parseSummary(data)
	if err != nil {
		return 0, 0, false, err
	}
	passed := p95 < p95ThresholdMs && errorRate < errorRateThreshold
	return p95, errorRate, passed, nil
}

func parseSummary(data []byte) (float64, float64, error) {
	var summary struct {
		Metrics map[string]struct {
			Values map[string]float64 `json:"values"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		return 0, 0, fmt.Errorf("parse summary.json: %w", err)
	}

	durationMetric, ok := summary.Metrics["http_req_duration{expected_response:true}"]
	if !ok {
		durationMetric, ok = summary.Metrics["http_req_duration"]
		if !ok {
			return 0, 0, fmt.Errorf("summary missing http_req_duration metric")
		}
	}
	p95, ok := durationMetric.Values["p(95)"]
	if !ok {
		return 0, 0, fmt.Errorf("summary missing p(95) value")
	}

	failedMetric, ok := summary.Metrics["http_req_failed"]
	if !ok {
		return 0, 0, fmt.Errorf("summary missing http_req_failed metric")
	}
	errorRate, ok := failedMetric.Values["rate"]
	if !ok {
		return 0, 0, fmt.Errorf("summary missing http_req_failed rate")
	}

	return p95, errorRate, nil
}

func runSuite(ctx context.Context, logger *slog.Logger, cfg config, backendDir string) error {
	runID := time.Now().UTC().Format("20060102-150405.000000")
	resultsDir := filepath.Join(backendDir, "perf", "results", "endpoints", runID)
	if err := os.MkdirAll(resultsDir, 0o750); err != nil {
		return fmt.Errorf("create results directory: %w", err)
	}

	results := make([]endpointResult, 0, len(endpointManifest))
	anyFailed := false

	for _, endpoint := range endpointManifest {
		maxRPS, p95, errorRate, passed, err := benchmarkEndpoint(ctx, logger, endpoint, cfg, backendDir)
		res := endpointResult{
			Endpoint:  endpoint,
			MaxRPS:    maxRPS,
			P95MS:     p95,
			ErrorRate: errorRate,
			Passed:    passed,
		}
		if err != nil {
			anyFailed = true
			res.Passed = false
			res.Error = err.Error()
			logger.ErrorContext(ctx, "endpoint benchmark failed", slog.String("endpoint", endpoint), slog.String("error", err.Error()))
		} else if !passed {
			anyFailed = true
		}
		results = append(results, res)
	}

	report := suiteResults{
		RunID:      runID,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		APIBaseURL: cfg.apiBaseURL,
		Duration:   cfg.duration.String(),
		StartRate:  cfg.startRate,
		MaxRate:    cfg.maxRate,
		Endpoints:  results,
	}

	if err := writeResultsJSON(resultsDir, report); err != nil {
		return err
	}
	if err := writeReportMD(resultsDir, report); err != nil {
		return err
	}

	logger.InfoContext(ctx, "suite report written",
		slog.String("results_dir", resultsDir),
		slog.Int("endpoints", len(results)),
		slog.Bool("all_passed", !anyFailed),
	)

	if anyFailed {
		return fmt.Errorf("one or more endpoints failed the benchmark")
	}
	return nil
}

func writeResultsJSON(resultsDir string, report suiteResults) error {
	path := filepath.Join(resultsDir, "results.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write results.json: %w", err)
	}
	return nil
}

func writeReportMD(resultsDir string, report suiteResults) error {
	path := filepath.Join(resultsDir, "report.md")
	var b strings.Builder

	fmt.Fprintf(&b, "# Performance Benchmark Report\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", report.RunID)
	fmt.Fprintf(&b, "- API Base URL: %s\n", report.APIBaseURL)
	fmt.Fprintf(&b, "- Duration per step: %s\n", report.Duration)
	fmt.Fprintf(&b, "- Start Rate: %d RPS\n", report.StartRate)
	fmt.Fprintf(&b, "- Max Rate: %d RPS\n\n", report.MaxRate)

	fmt.Fprintf(&b, "| Endpoint | Max RPS | P95 (ms) | Error Rate | Status |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|\n")

	for _, r := range report.Endpoints {
		status := "PASS"
		if !r.Passed {
			if r.Error != "" {
				status = "ERROR"
			} else {
				status = "FAIL"
			}
		}
		fmt.Fprintf(&b, "| %s | %d | %.2f | %.4f | %s |\n", r.Endpoint, r.MaxRPS, r.P95MS, r.ErrorRate, status)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write report.md: %w", err)
	}
	return nil
}
