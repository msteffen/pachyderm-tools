package cmds

import (
	"testing"
)

// This test suite confirms that the regular expressions used by 'changed.go'
// (which are complex) match status lines as emitted by git

func TestStatusBasic(t *testing.T) {
	status := ` M src/server/pps/server/worker_rc.go`
	c := plainStatusLineRe.FindStringSubmatch(status)
	if c == nil {
		t.Fatalf("%q should have matched %q but didn't", status, plainStatusLineRe)
	}
	if len(c) != 3 {
		t.Fatalf("expected 3 capture groups but got %d", len(c))
	}
	expectedFilename := "src/server/pps/server/worker_rc.go"
	if c[2] != expectedFilename {
		t.Fatalf("expected filename to be %q, but was %q", expectedFilename, c[2])
	}
}

func TestStatusArrow(t *testing.T) {
	status := `R  doc/managing_pachyderm/sharing-gpu-resources.md -> doc/managing_pachyderm/sharing_gpu_resources.md`
	c := arrowStatusLineRe.FindStringSubmatch(status)
	if c == nil {
		t.Fatalf("%q should have matched %q but didn't", status, arrowStatusLineRe)
	}
	if len(c) != 3 {
		t.Fatalf("expected 3 capture groups but got %d", len(c))
	}
	expectedFilename := "doc/managing_pachyderm/sharing_gpu_resources.md"
	if c[2] != expectedFilename {
		t.Fatalf("expected filename to be %q, but was %q", expectedFilename, c[2])
	}
}

func TestStatusSpace(t *testing.T) {
	status := ` M "This is a test"`
	c := plainStatusLineRe.FindStringSubmatch(status)
	if c == nil {
		t.Fatalf("%q should have matched %q but didn't", status, plainStatusLineRe)
	}
	if len(c) != 3 {
		t.Fatalf("expected 3 capture groups but got %d", len(c))
	}
	expectedFilename := `"This is a test"`
	if c[2] != expectedFilename {
		t.Fatalf("expected filename to be %q, but was %q", expectedFilename, c[2])
	}
}

func TestStatusArrowSpace(t *testing.T) {
	status := `RM "This is a test" -> "This is also a \"test\""`
	c := arrowStatusLineRe.FindStringSubmatch(status)
	if c == nil {
		t.Fatalf("%q should have matched %q but didn't", status, arrowStatusLineRe)
	}
	if len(c) != 3 {
		t.Fatalf("expected 3 capture groups but got %d", len(c))
	}
	expectedFilename := `"This is also a \"test\""`
	if c[2] != expectedFilename {
		t.Fatalf("expected filename to be %q, but was %q", expectedFilename, c[2])
	}
}
