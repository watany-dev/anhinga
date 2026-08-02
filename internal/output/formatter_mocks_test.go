package output

import (
	"fmt"
)

// badWriter always fails on Write
type badWriter struct{}

func (w *badWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("always fails")
}
