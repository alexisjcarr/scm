package logging

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestNewTextLoggerIncludesExplicitTimestampAndStableKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	logger := New(Options{Level: "info", Out: &out})
	logger.Info("hello world", "component", "test")

	line := out.String()
	if !regexp.MustCompile(`timestamp=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`).MatchString(line) {
		t.Fatalf("expected RFC3339 timestamp in %q", line)
	}
	if !strings.Contains(line, "level=info") {
		t.Fatalf("expected stable level key in %q", line)
	}
	if !strings.Contains(line, "msg=\"hello world\"") {
		t.Fatalf("expected stable msg key in %q", line)
	}
}
