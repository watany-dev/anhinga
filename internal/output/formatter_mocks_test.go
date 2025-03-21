package output

import (
	"fmt"
	"io"
)

// mockWriter allows testing of error conditions in formatAsCSV
type mockWriter struct {
	writeCallCount int
	failOnCall     int
}

func newMockWriter(failOnCall int) *mockWriter {
	return &mockWriter{
		writeCallCount: 0,
		failOnCall:     failOnCall,
	}
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	w.writeCallCount++
	if w.writeCallCount == w.failOnCall {
		return 0, fmt.Errorf("simulated write error")
	}
	return len(p), nil
}

// badWriter always fails on Write
type badWriter struct{}

func (w *badWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("always fails")
}

// errorCSVWriter is a writer that meets just enough of the CSV writer interface to test error cases
type errorCSVWriter struct {
	w io.Writer
}

func (c *errorCSVWriter) Write(record []string) error {
	// Simulate writing the record
	_, err := fmt.Fprintln(c.w, record)
	return err
}

func (c *errorCSVWriter) Flush() {
	// Do nothing
}

func (c *errorCSVWriter) Error() error {
	// No error
	return nil
}