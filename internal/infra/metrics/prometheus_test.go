package metrics

import "testing"

func TestPrometheusMetricsExposesPasswordChangeCleanupFailures(t *testing.T) {
	metrics := NewPrometheusMetrics()
	metrics.RecordPasswordChangeCleanupFailure("session_lookup")
	metrics.RecordPasswordChangeCleanupFailure("session_lookup")
	metrics.RecordPasswordChangeCleanupFailure("session_revocation")
	metrics.RecordPasswordChangeCleanupFailure("session_validation")

	families, err := metrics.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	counts := make(map[string]float64)
	for _, family := range families {
		if family.GetName() != "password_change_cleanup_failures_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			var stage string
			for _, label := range metric.GetLabel() {
				if label.GetName() == "stage" {
					stage = label.GetValue()
				}
			}
			counts[stage] = metric.GetCounter().GetValue()
		}
	}

	if counts["session_lookup"] != 2 {
		t.Fatalf("session_lookup counter = %v, want 2", counts["session_lookup"])
	}
	if counts["session_revocation"] != 1 {
		t.Fatalf("session_revocation counter = %v, want 1", counts["session_revocation"])
	}
	if counts["session_validation"] != 1 {
		t.Fatalf("session_validation counter = %v, want 1", counts["session_validation"])
	}
}
