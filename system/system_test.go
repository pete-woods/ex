package system

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/circleci/ex/o11y"
	"github.com/circleci/ex/o11y/honeycomb"
	"github.com/circleci/ex/termination"
	"github.com/circleci/ex/testing/fakemetrics"
)

func TestSystem_Run(t *testing.T) {
	metrics := &fakemetrics.Provider{}
	ctx := o11y.WithProvider(context.Background(), honeycomb.New(honeycomb.Config{
		Format:  "color",
		Metrics: metrics,
	}))

	// Wait until everything has been exercised before terminating
	terminationWait := &sync.WaitGroup{}
	terminationTestHook = func(ctx context.Context, delay time.Duration) error {
		terminationWait.Wait()
		return termination.ErrTerminated
	}

	sys := New()

	sys.AddMetrics(newMockMetricProducer(terminationWait))

	terminationWait.Add(1)
	sys.AddService(func(ctx context.Context) (err error) {
		ctx, span := o11y.StartSpan(ctx, "service")
		defer o11y.End(span, &err)
		terminationWait.Done()
		<-ctx.Done()
		return nil
	})

	sys.AddHealthCheck(newMockHealthChecker())

	cleanupCalled := false
	sys.AddCleanup(func(ctx context.Context) (err error) {
		ctx, span := o11y.StartSpan(ctx, "cleanup")
		defer o11y.End(span, &err)
		cleanupCalled = true
		return nil
	})

	err := sys.Run(ctx, 0)
	assert.Check(t, errors.Is(err, termination.ErrTerminated))

	sys.Cleanup(ctx)
	assert.Check(t, cleanupCalled)

	assert.Check(t, cmp.DeepEqual(metrics.Calls(), []fakemetrics.MetricCall{
		{
			Metric: "gauge",
			Name:   "gauge..key_a",
			Value:  1,
			Tags:   []string{"foo:bar"},
			Rate:   1,
		},
		{
			Metric: "gauge",
			Name:   "gauge..key_b",
			Value:  2,
			Tags:   []string{"baz:qux"},
			Rate:   1,
		},
		{
			Metric: "timer",
			Name:   "worker_loop",
			Value:  0.01,
			Tags:   []string{"loop_name:metric-loop", "result:success"},
			Rate:   1,
		},
		{
			Metric: "timer",
			Name:   "system.run",
			Value:  0.3,
			Tags:   []string{"result:success"},
			Rate:   1,
		},
	}, fakemetrics.CMPMetrics))
}

type mockMetricProducer struct {
	wg *sync.WaitGroup
}

func newMockMetricProducer(wg *sync.WaitGroup) *mockMetricProducer {
	wg.Add(3)
	return &mockMetricProducer{wg: wg}
}

func (m *mockMetricProducer) MetricName() string {
	m.wg.Done()
	return ""
}

func (m *mockMetricProducer) Gauges(_ context.Context) map[string]float64 {
	m.wg.Done()
	return map[string]float64{
		"key_a": 1,
		"key_b": 2,
	}
}

func (m *mockMetricProducer) Tags(_ context.Context) map[string][]string {
	m.wg.Done()
	return map[string][]string{
		"key_a": {"foo:bar"},
		"key_b": {"baz:qux"},
	}
}

type mockHealthChecker struct {
}

func newMockHealthChecker() *mockHealthChecker {
	return &mockHealthChecker{}
}

func (m *mockHealthChecker) HealthChecks() (name string, ready, live func(ctx context.Context) error) {
	return "name", nil, nil
}
