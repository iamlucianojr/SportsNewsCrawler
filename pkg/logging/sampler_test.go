package logging

import (
	"testing"
)

func TestErrorSampler(t *testing.T) {
	sampler := NewErrorSampler(10)

	// First occurrence should be logged
	if !sampler.ShouldLog("test_error") {
		t.Error("First occurrence should be logged")
	}

	// Occurrences 2-9 should not be logged
	for i := 2; i <= 9; i++ {
		if sampler.ShouldLog("test_error") {
			t.Errorf("Occurrence %d should not be logged", i)
		}
	}

	// 10th occurrence should be logged
	if !sampler.ShouldLog("test_error") {
		t.Error("10th occurrence should be logged")
	}

	// Verify count
	if count := sampler.GetCount("test_error"); count != 10 {
		t.Errorf("Expected count 10, got %d", count)
	}

	// Test reset
	sampler.Reset("test_error")
	if count := sampler.GetCount("test_error"); count != 0 {
		t.Errorf("Expected count 0 after reset, got %d", count)
	}
}

func TestErrorSamplerMultipleKeys(t *testing.T) {
	sampler := NewErrorSampler(5)

	// Different error types should be tracked independently
	sampler.ShouldLog("error_a")
	sampler.ShouldLog("error_b")

	if sampler.GetCount("error_a") != 1 {
		t.Error("error_a count should be 1")
	}
	if sampler.GetCount("error_b") != 1 {
		t.Error("error_b count should be 1")
	}

	// Reset all
	sampler.ResetAll()
	if sampler.GetCount("error_a") != 0 || sampler.GetCount("error_b") != 0 {
		t.Error("All counts should be 0 after ResetAll")
	}
}
