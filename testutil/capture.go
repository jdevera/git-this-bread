package testutil

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// CaptureStdout captures stdout during the execution of fn.
func CaptureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
	}()

	fn()

	_ = w.Close()
	wg.Wait()
	os.Stdout = old

	return buf.String()
}
