package payment

import (
	"context"
	"math"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newTestMetrics installs an in-memory OpenTelemetry MeterProvider backed by a
// manual reader, builds the Metrics under test from it, and returns both.
// The previous global MeterProvider is restored on cleanup.
//
// The caller MUST NOT call t.Parallel: this helper mutates the global
// OpenTelemetry MeterProvider.
func newTestMetrics(t *testing.T) (*Metrics, *sdkmetric.ManualReader) {
	t.Helper()

	prev := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})

	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	return m, reader
}

func TestRecordRequest(t *testing.T) {
	// NOTE: no t.Parallel — this test mutates the global MeterProvider.
	m, reader := newTestMetrics(t)

	ctx := t.Context()

	// Durations are chosen so that .Seconds() equals exactly 0.123 and 0.05,
	// making the expected histogram sum deterministic: 0.123 + 0.05 = 0.173.
	const (
		durOK  = 123 * time.Millisecond
		durErr = 50 * time.Millisecond
	)
	wantLatencySum := durOK.Seconds() + durErr.Seconds()

	m.RecordRequest(ctx, "fake", "InitPayment", "ok", durOK)
	m.RecordRequest(ctx, "fake", "InitPayment", "error", durErr)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	t.Run("requests counter totals 2 across both statuses", func(t *testing.T) {
		sum := sumMetric(t, &rm, "payment.provider.requests")
		if got := totalValue(sum.DataPoints); got != 2 {
			t.Fatalf("requests total = %d, want 2", got)
		}
		assertStatusValue(t, sum.DataPoints, "ok", 1)
		assertStatusValue(t, sum.DataPoints, "error", 1)
	})

	t.Run("errors counter increments only on error status", func(t *testing.T) {
		sum := sumMetric(t, &rm, "payment.provider.errors")
		if got := totalValue(sum.DataPoints); got != 1 {
			t.Fatalf("errors total = %d, want 1", got)
		}
		assertStatusValue(t, sum.DataPoints, "error", 1)
		if _, ok := dataPointByStatus(sum.DataPoints, "ok"); ok {
			t.Fatal("errors counter must not record an ok status series")
		}
	})

	t.Run("latency histogram aggregates count and sum", func(t *testing.T) {
		hist := histMetric(t, &rm, "payment.provider.latency")
		if m, ok := metricByName(&rm, "payment.provider.latency"); !ok || m.Unit != "s" {
			t.Errorf("latency histogram unit = %q, want %q", m.Unit, "s")
		}
		var count uint64
		var sum float64
		for _, dp := range hist.DataPoints {
			count += dp.Count
			sum += dp.Sum
		}
		if count != 2 {
			t.Errorf("latency count = %d, want 2", count)
		}
		if math.Abs(sum-wantLatencySum) > 1e-12 {
			t.Errorf("latency sum = %v, want %v (0.123 + 0.05)", sum, wantLatencySum)
		}
	})

	t.Run("attributes are low-cardinality (provider/operation/status only)", func(t *testing.T) {
		for _, name := range []string{
			"payment.provider.requests",
			"payment.provider.errors",
		} {
			for _, dp := range sumMetric(t, &rm, name).DataPoints {
				assertLowCardinalityKeys(t, dp.Attributes)
			}
		}
		for _, dp := range histMetric(t, &rm, "payment.provider.latency").DataPoints {
			assertLowCardinalityKeys(t, dp.Attributes)
		}
	})
}

func TestRecordRequest_NilSafe(t *testing.T) {
	t.Parallel() // does not touch the global MeterProvider

	var m *Metrics
	// Must not panic on a nil receiver.
	m.RecordRequest(t.Context(), "fake", "InitPayment", "ok", 100*time.Millisecond)
}

func sumMetric(t *testing.T, rm *metricdata.ResourceMetrics, name string) metricdata.Sum[int64] {
	t.Helper()
	m, ok := metricByName(rm, name)
	if !ok {
		t.Fatalf("metric %q not found in collected ResourceMetrics", name)
	}
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("metric %q Data type = %T, want metricdata.Sum[int64]", name, m.Data)
	}
	return sum
}

func histMetric(t *testing.T, rm *metricdata.ResourceMetrics, name string) metricdata.Histogram[float64] {
	t.Helper()
	m, ok := metricByName(rm, name)
	if !ok {
		t.Fatalf("metric %q not found in collected ResourceMetrics", name)
	}
	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("metric %q Data type = %T, want metricdata.Histogram[float64]", name, m.Data)
	}
	return hist
}

func metricByName(rm *metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}
	return metricdata.Metrics{}, false
}

func totalValue(dps []metricdata.DataPoint[int64]) int64 {
	var total int64
	for _, dp := range dps {
		total += dp.Value
	}
	return total
}

// dataPointByStatus returns the data point whose "status" attribute equals status.
func dataPointByStatus(dps []metricdata.DataPoint[int64], status string) (metricdata.DataPoint[int64], bool) {
	for _, dp := range dps {
		if v, ok := dp.Attributes.Value("status"); ok && v.AsString() == status {
			return dp, true
		}
	}
	return metricdata.DataPoint[int64]{}, false
}

func assertStatusValue(t *testing.T, dps []metricdata.DataPoint[int64], status string, want int64) {
	t.Helper()
	dp, ok := dataPointByStatus(dps, status)
	if !ok {
		t.Fatalf("no data point with status=%q", status)
	}
	if dp.Value != want {
		t.Errorf("data point status=%q value = %d, want %d", status, dp.Value, want)
	}
}

func assertLowCardinalityKeys(t *testing.T, set attribute.Set) {
	t.Helper()
	kvs := set.ToSlice()
	keys := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		keys = append(keys, string(kv.Key))
	}
	slices.Sort(keys)
	want := []string{"operation", "provider", "status"}
	if !slices.Equal(keys, want) {
		t.Errorf("attribute keys = %v, want exactly %v (no payment IDs/amounts)", keys, want)
	}
	if v, ok := set.Value("provider"); !ok || v.AsString() != "fake" {
		t.Errorf("provider attribute = %q, want %q", v.AsString(), "fake")
	}
	if v, ok := set.Value("operation"); !ok || v.AsString() != "InitPayment" {
		t.Errorf("operation attribute = %q, want %q", v.AsString(), "InitPayment")
	}
}
