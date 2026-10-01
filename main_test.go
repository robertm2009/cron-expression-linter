package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunJSON(t *testing.T) {
	input := "0 3 * * * ok\n61 5 * * * echo hi\n0 9 15 * mon echo hi\n"
	code, out, _ := runCLI(t, input, "--json", "-")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	var got []jsonFinding
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(got), got)
	}
	if got[0].File != "stdin" || got[0].Line != 2 || got[0].Column != 1 || got[0].EndColumn != 3 || got[0].Severity != "error" {
		t.Errorf("unexpected first finding: %+v", got[0])
	}
	if got[1].Line != 3 || got[1].Severity != "warning" {
		t.Errorf("unexpected second finding: %+v", got[1])
	}
}

func TestRunJSONNoFindings(t *testing.T) {
	code, out, _ := runCLI(t, "0 3 * * * ok\n", "--json", "-")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("output = %q, want []", out)
	}
}

func TestRunWarningsDoNotFail(t *testing.T) {
	code, _, _ := runCLI(t, "0 9 15 * mon echo hi\n", "-")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunTextOutput(t *testing.T) {
	code, out, _ := runCLI(t, "61 5 * * * echo hi\n", "-")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(out, "stdin:1:1: error: ") {
		t.Errorf("unexpected text output: %q", out)
	}
}

func TestRunUsageErrors(t *testing.T) {
	if code, _, _ := runCLI(t, ""); code != 2 {
		t.Errorf("no args: exit code = %d, want 2", code)
	}
	if code, _, _ := runCLI(t, "", "--bogus", "-"); code != 2 {
		t.Errorf("bad flag: exit code = %d, want 2", code)
	}
	if code, _, errOut := runCLI(t, "", "/nonexistent/crontab"); code != 2 || !strings.Contains(errOut, "/nonexistent/crontab") {
		t.Errorf("missing file: exit code = %d, stderr = %q", code, errOut)
	}
}
