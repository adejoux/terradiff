package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run go test -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s (run go test -update to accept):\n%s", path, got)
	}
}

func TestGolden(t *testing.T) {
	demo := []string{"testdata/old.json", "testdata/new.json"}
	// Real schemas of hashicorp/random, generated with
	// `terraform providers schema -json`.
	random := []string{"testdata/random/v3.1.0.json", "testdata/random/v3.6.3.json"}
	tests := []struct {
		golden string
		args   []string
		files  []string
	}{
		{"release-notes.md", []string{"--old-version", "v1.0.0", "--new-version", "v2.0.0"}, demo},
		{"release-notes-descriptions.md", []string{"--descriptions", "--heading-level", "2", "--title", "v2.0.0"}, demo},
		{"report.json", []string{"--format", "json"}, demo},
		{"random/release-notes.md", []string{"--old-version", "v3.1.0", "--new-version", "v3.6.3"}, random},
		{"common/release-notes.md", []string{"--new-version", "v2.0.0"}, []string{"testdata/common/old.json", "testdata/common/new.json"}},
		{"common/release-notes-no-common.md", []string{"--new-version", "v2.0.0", "--common-min", "0"}, []string{"testdata/common/old.json", "testdata/common/new.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			var out, errOut bytes.Buffer
			args := append(tt.args, tt.files...)
			code, err := run(args, &out, &errOut)
			if err != nil || code != 0 {
				t.Fatalf("run: code=%d err=%v stderr=%s", code, err, errOut.String())
			}
			golden(t, tt.golden, out.Bytes())
		})
	}
}

func TestNoChanges(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := run([]string{"--fail-on-breaking", "testdata/old.json", "testdata/old.json"}, &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if !strings.Contains(out.String(), "No schema changes.") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}

func TestFailOnBreaking(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := run([]string{"testdata/old.json", "testdata/new.json", "--fail-on-breaking"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != exitBreaking {
		t.Errorf("exit code = %d, want %d", code, exitBreaking)
	}
}

func TestBadArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	if code, _ := run([]string{"testdata/old.json"}, &out, &errOut); code != 1 {
		t.Errorf("one file: exit code = %d, want 1", code)
	}
	if code, _ := run([]string{"--format", "xml", "testdata/old.json", "testdata/new.json"}, &out, &errOut); code != 1 {
		t.Errorf("bad format: exit code = %d, want 1", code)
	}
	if code, _ := run([]string{"--provider", "nope", "testdata/old.json", "testdata/new.json"}, &out, &errOut); code != 1 {
		t.Errorf("unknown provider: exit code = %d, want 1", code)
	}
}
