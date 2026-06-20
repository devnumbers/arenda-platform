package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/diagnostics"
	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/fixtures"
	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/model"
	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/report"
	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/vegeta"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(os.Args) < 2 {
		fatalf("usage: perfvegeta sustainable|breakdown [-endpoint name]")
	}

	mode := os.Args[1]
	if mode != "sustainable" && mode != "breakdown" {
		fatalf("unknown mode %q; expected sustainable or breakdown", mode)
	}

	flags := flag.NewFlagSet(mode, flag.ExitOnError)
	endpointName := flags.String("endpoint", "", "read endpoint name to test; empty means all read endpoints")
	if err := flags.Parse(os.Args[2:]); err != nil {
		fatalf("parse flags: %v", err)
	}

	profile := defaultGeneratorProfile()
	if profile.Connections > profile.MaxConnections {
		fatalf("invalid generator profile: connections=%d exceeds max_connections=%d", profile.Connections, profile.MaxConnections)
	}

	if _, err := exec.LookPath("vegeta"); err != nil {
		fatalf("vegeta binary not found in PATH: %v", err)
	}

	fx, err := fixtures.Load(model.FixturesPath)
	if err != nil {
		fatalf("load fixtures: %v", err)
	}

	host := diagnostics.DetectHostLimits(ctx, profile)
	diagnostics.PrintHostDiagnostics(host, profile)

	targets := selectEndpoints(*endpointName)
	if len(targets) == 0 {
		fatalf("endpoint %q not found", *endpointName)
	}

	reportDir := filepath.Join(model.ResultsDir, "run_"+time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(reportDir, 0o700); err != nil {
		fatalf("create report dir: %v", err)
	}

	client := newHTTPClient(profile)
	cfg := model.RunConfig{
		Mode:      mode,
		Endpoint:  *endpointName,
		BaseURL:   appBaseURL(),
		Fixtures:  fx,
		Generator: profile,
		Host:      host,
		ReportDir: reportDir,
	}

	var summaries []model.EndpointSummary
	for _, ep := range targets {
		var summary model.EndpointSummary
		switch mode {
		case "sustainable":
			summary = searchSustainable(ctx, cfg, ep, client)
		case "breakdown":
			summary = scanBreakdown(ctx, cfg, ep, client)
		}
		summaries = append(summaries, summary)
	}

	jsonPath, csvPath, err := report.WriteReports(reportDir, summaries)
	if err != nil {
		fatalf("write reports: %v", err)
	}

	report.PrintSummaryTable(summaries)
	fmt.Printf("\nreports:\n  json: %s\n  csv:  %s\n", jsonPath, csvPath)
}

func newHTTPClient(profile model.GeneratorProfile) *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        profile.MaxConnections,
			MaxIdleConnsPerHost: profile.MaxConnections,
			MaxConnsPerHost:     profile.MaxConnections,
			IdleConnTimeout:     30 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   1 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}
}

func appBaseURL() string {
	if value := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/"); value != "" {
		return value
	}
	return model.DefaultBaseURL
}

func defaultGeneratorProfile() model.GeneratorProfile {
	return model.GeneratorProfile{
		StartRate:      model.StartRate,
		MaxRate:        model.MaxRate,
		RateStep:       model.RateStep,
		Tolerance:      model.Tolerance,
		Duration:       model.BenchmarkDuration,
		DurationText:   model.BenchmarkDuration.String(),
		Timeout:        model.RequestTimeout,
		TimeoutText:    model.RequestTimeout.String(),
		ProbeCooldown:  model.ProbeCooldown,
		CooldownText:   model.ProbeCooldown.String(),
		Confirmations:  model.SustainableConfirmationAttempts,
		Connections:    model.VegetaConnections,
		MaxConnections: model.VegetaMaxConnections,
		Workers:        model.VegetaWorkers,
		MaxWorkers:     model.VegetaMaxWorkers,
		MaxBody:        model.VegetaMaxBody,
	}
}

func selectEndpoints(name string) []model.Endpoint {
	if name == "" {
		return model.Endpoints
	}

	for _, ep := range model.Endpoints {
		if ep.Name == name {
			return []model.Endpoint{ep}
		}
	}
	return nil
}

func searchSustainable(ctx context.Context, cfg model.RunConfig, ep model.Endpoint, client *http.Client) model.EndpointSummary {
	probes := make([]model.ProbeResult, 0, 16)

	start := vegeta.Attack(ctx, cfg, ep, cfg.Generator.StartRate, client)
	if start.Bottleneck == "generator" {
		start.Status = "invalid_generator_limit"
		probes = append(probes, start)
		return summaryFromProbe(ep, cfg, start, probes, nil, "invalid_generator_limit", false)
	}
	if !start.Pass {
		start.Status = "no_sustainable_rate"
		probes = append(probes, start)
		return summaryFromProbe(ep, cfg, start, probes, nil, "no_sustainable_rate", false)
	}
	probes = append(probes, start)

	best := start
	low := cfg.Generator.StartRate
	high := cfg.Generator.MaxRate
	var limiter *model.ProbeResult

	for high-low > cfg.Generator.Tolerance {
		if !waitForProbeCooldown(ctx, cfg) {
			return summaryFromProbe(ep, cfg, best, probes, limiter, "canceled", false)
		}

		mid := (low + high) / 2
		probe := vegeta.Attack(ctx, cfg, ep, mid, client)
		probes = append(probes, probe)

		if probe.Pass {
			best = probe
			low = mid
			continue
		}

		limiter = &probe
		high = mid
	}

	status := "sustainable_limit"
	pass := true
	summaryProbe := best
	if limiter == nil {
		status = "max_rate_reached"
	} else if limiter.Bottleneck == "generator" {
		status = "invalid_generator_limit"
		pass = false
		summaryProbe = *limiter
	}

	confirmation := confirmationSummary{}
	if pass && best.Pass {
		confirmation = confirmationSummary{Attempts: 1, Passes: 1, ConfirmedRate: best.RequestedRate}
		if cfg.Generator.Confirmations > 1 {
			var failed *model.ProbeResult
			confirmation, probes, failed = confirmSustainableCandidate(ctx, cfg, ep, best, probes, client)
			if failed != nil {
				pass = false
				limiter = failed
				status = "unstable_sustainable_limit"
				summaryProbe = *failed
				if failed.Bottleneck == "generator" {
					status = "invalid_generator_limit"
				}
			}
		}
	}

	summary := summaryFromProbe(ep, cfg, summaryProbe, probes, limiter, status, pass && best.Pass)
	summary.ConfirmationAttempts = confirmation.Attempts
	summary.ConfirmationPasses = confirmation.Passes
	if pass && best.Pass {
		summary.ConfirmedRate = confirmation.ConfirmedRate
	}
	return summary
}

func scanBreakdown(ctx context.Context, cfg model.RunConfig, ep model.Endpoint, client *http.Client) model.EndpointSummary {
	probes := make([]model.ProbeResult, 0, 16)
	var lastSuccess *model.ProbeResult

	for rate := cfg.Generator.StartRate; rate <= cfg.Generator.MaxRate; rate += cfg.Generator.RateStep {
		probe := vegeta.Attack(ctx, cfg, ep, rate, client)

		if probe.Bottleneck == "generator" {
			probe.Status = "invalid_generator_limit"
			probes = append(probes, probe)
			return summaryFromProbe(ep, cfg, probe, probes, lastSuccess, "invalid_generator_limit", false)
		}

		if probe.Success <= model.BreakdownSuccessThreshold {
			probe.Status = "first_broken"
			probes = append(probes, probe)
			return summaryFromProbe(ep, cfg, probe, probes, lastSuccess, "first_broken", false)
		}

		probes = append(probes, probe)
		lastSuccess = copyProbe(probe)
		if rate+cfg.Generator.RateStep <= cfg.Generator.MaxRate && !waitForProbeCooldown(ctx, cfg) {
			return summaryFromProbe(ep, cfg, probe, probes, lastSuccess, "canceled", false)
		}
	}

	if lastSuccess == nil {
		last := probes[len(probes)-1]
		return summaryFromProbe(ep, cfg, last, probes, nil, "no_result", false)
	}

	return summaryFromProbe(ep, cfg, *lastSuccess, probes, lastSuccess, "max_rate_reached", true)
}

type confirmationSummary struct {
	Attempts      int
	Passes        int
	ConfirmedRate uint64
}

func confirmSustainableCandidate(ctx context.Context, cfg model.RunConfig, ep model.Endpoint, candidate model.ProbeResult, probes []model.ProbeResult, client *http.Client) (confirmationSummary, []model.ProbeResult, *model.ProbeResult) {
	confirmation := confirmationSummary{
		Attempts:      1,
		ConfirmedRate: candidate.RequestedRate,
	}
	if candidate.Pass {
		confirmation.Passes = 1
	}

	var firstFailure *model.ProbeResult
	for attempt := 2; attempt <= cfg.Generator.Confirmations; attempt++ {
		if !waitForProbeCooldown(ctx, cfg) {
			canceled := candidate
			canceled.Status = "canceled"
			canceled.Pass = false
			canceled.Bottleneck = "unknown"
			canceled.BottleneckCause = "run canceled during sustainable confirmation cooldown"
			return confirmation, probes, &canceled
		}

		probe := vegeta.Attack(ctx, cfg, ep, candidate.RequestedRate, client)
		confirmation.Attempts++
		if probe.Pass {
			probe.Status = "confirmation_passed"
			confirmation.Passes++
		} else if probe.Bottleneck != "generator" {
			probe.Status = "confirmation_failed"
		}
		probes = append(probes, probe)

		if !probe.Pass && firstFailure == nil {
			firstFailure = copyProbe(probe)
		}
	}

	if confirmation.Passes != confirmation.Attempts {
		return confirmation, probes, firstFailure
	}
	return confirmation, probes, nil
}

func waitForProbeCooldown(ctx context.Context, cfg model.RunConfig) bool {
	if cfg.Generator.ProbeCooldown <= 0 {
		return true
	}

	fmt.Printf("cooldown %s before next probe\n", cfg.Generator.CooldownText)
	timer := time.NewTimer(cfg.Generator.ProbeCooldown)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func summaryFromProbe(ep model.Endpoint, cfg model.RunConfig, probe model.ProbeResult, probes []model.ProbeResult, limiterOrLastSuccess *model.ProbeResult, status string, pass bool) model.EndpointSummary {
	summaryProbe := probe
	summaryProbe.Status = status
	markers := summarizeProbeMarkers(probes)

	summary := model.EndpointSummary{
		Endpoint:             ep.Name,
		Mode:                 cfg.Mode,
		Status:               status,
		RequestedRate:        summaryProbe.RequestedRate,
		VegetaRate:           summaryProbe.VegetaRate,
		Throughput:           summaryProbe.Throughput,
		ThroughputRatio:      summaryProbe.ThroughputRatio,
		Success:              summaryProbe.Success,
		P95MS:                summaryProbe.P95MS,
		P99MS:                summaryProbe.P99MS,
		MaxMS:                summaryProbe.MaxMS,
		WaitSec:              summaryProbe.WaitSec,
		Requests:             summaryProbe.Requests,
		Pass:                 pass,
		PoolWarning:          summaryProbe.PoolWarning,
		PoolWarningCause:     summaryProbe.PoolWarningCause,
		Bottleneck:           summaryProbe.Bottleneck,
		BottleneckCause:      summaryProbe.BottleneckCause,
		Errors:               summaryProbe.Errors,
		UnstableBoundary:     markers.UnstableBoundary,
		Generator:            cfg.Generator,
		Host:                 cfg.Host,
		BestSLOPass:          markers.BestSLOPass,
		BestPass:             markers.BestSLOPass,
		Baseline5000Probe:    markers.Baseline5000Probe,
		FirstSLOFailObserved: markers.FirstSLOFailObserved,
		FirstSLOFail:         markers.FirstSLOFailObserved,
		FirstGeneratorFail:   markers.FirstGeneratorFail,
		Probes:               probes,
	}

	switch cfg.Mode {
	case "sustainable":
		if limiterOrLastSuccess != nil {
			summary.Bottleneck = limiterOrLastSuccess.Bottleneck
			summary.BottleneckCause = limiterOrLastSuccess.BottleneckCause
		}
	case "breakdown":
		if limiterOrLastSuccess != nil {
			summary.LastSuccessRate = limiterOrLastSuccess.RequestedRate
			summary.LastGoodRate = limiterOrLastSuccess.RequestedRate
		}
		if status == "first_broken" || status == "invalid_generator_limit" {
			summary.FirstBrokenRate = summaryProbe.RequestedRate
		}
	}

	return summary
}

type probeMarkers struct {
	BestSLOPass          *model.ProbeResult
	Baseline5000Probe    *model.ProbeResult
	FirstSLOFailObserved *model.ProbeResult
	FirstGeneratorFail   *model.ProbeResult
	UnstableBoundary     bool
}

func summarizeProbeMarkers(probes []model.ProbeResult) probeMarkers {
	var markers probeMarkers

	for i := range probes {
		probe := probes[i]
		if probe.RequestedRate == model.BaselineRate && markers.Baseline5000Probe == nil {
			markers.Baseline5000Probe = copyProbe(probe)
		}
		switch {
		case probe.Pass:
			if markers.BestSLOPass == nil || probe.RequestedRate > markers.BestSLOPass.RequestedRate {
				markers.BestSLOPass = copyProbe(probe)
			}
		case probe.Bottleneck == "generator":
			if markers.FirstGeneratorFail == nil || probe.RequestedRate < markers.FirstGeneratorFail.RequestedRate {
				markers.FirstGeneratorFail = copyProbe(probe)
			}
		default:
			if markers.FirstSLOFailObserved == nil || probe.RequestedRate < markers.FirstSLOFailObserved.RequestedRate {
				markers.FirstSLOFailObserved = copyProbe(probe)
			}
		}
	}

	markers.UnstableBoundary = markers.BestSLOPass != nil &&
		markers.FirstSLOFailObserved != nil &&
		markers.BestSLOPass.RequestedRate > markers.FirstSLOFailObserved.RequestedRate
	return markers
}

func copyProbe(probe model.ProbeResult) *model.ProbeResult {
	copied := probe
	return &copied
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
