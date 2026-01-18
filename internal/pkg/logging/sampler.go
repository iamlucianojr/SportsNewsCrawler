package logging

import (
	"sync"
)

// ErrorSampler reduces log noise by sampling repeated errors
// Logs the first occurrence, then every Nth occurrence
type ErrorSampler struct {
	mu       sync.RWMutex
	counts   map[string]int
	interval int // Log every Nth occurrence
}

// NewErrorSampler creates a new error sampler
// interval: log every Nth occurrence (e.g., 10 means log 1st, 11th, 21st, etc.)
func NewErrorSampler(interval int) *ErrorSampler {
	if interval < 1 {
		interval = 10 // Default to every 10th
	}
	return &ErrorSampler{
		counts:   make(map[string]int),
		interval: interval,
	}
}

// ShouldLog returns true if this error should be logged
// errorKey: unique identifier for the error type (e.g., "kafka_connection_error")
func (s *ErrorSampler) ShouldLog(errorKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counts[errorKey]++
	count := s.counts[errorKey]

	// Log first occurrence and every Nth occurrence
	return count == 1 || count%s.interval == 0
}

// GetCount returns the current count for an error key
func (s *ErrorSampler) GetCount(errorKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.counts[errorKey]
}

// Reset clears the count for a specific error key
func (s *ErrorSampler) Reset(errorKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.counts, errorKey)
}

// ResetAll clears all error counts
func (s *ErrorSampler) ResetAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts = make(map[string]int)
}
