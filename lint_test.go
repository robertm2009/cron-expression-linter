package main

import (
	"testing"
)

// findAt returns the finding whose column matches col, or nil if there
// isn't one. Tests use this instead of indexing findings by position
// because a line can produce findings in a different order than the
// fields appear (e.g. the day-of-month/day-of-week warning comes last).
func findAt(findings []Finding, col int) *Finding {
	for i := range findings {
		if findings[i].Column == col {
			return &findings[i]
		}
	}
	return nil
}

func TestLintLineValid(t *testing.T) {
	cases := []string{
		"0 3 * * * /usr/local/bin/backup.sh",
		"*/15 * * * * echo hi",
		"0 0 * JAN sun echo hi",
		"0 0 1,15 * * echo hi",
		"0-30/10 * * * * echo hi",
	}
	for _, line := range cases {
		if got := LintLine(line, 1); got != nil {
			t.Errorf("LintLine(%q) = %v, want no findings", line, got)
		}
	}
}

func TestLintLineIgnoredLines(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"# a comment",
		"  # indented comment",
		"MAILTO=root",
		"PATH=/usr/bin:/bin",
	}
	for _, line := range cases {
		if got := LintLine(line, 1); got != nil {
			t.Errorf("LintLine(%q) = %v, want nil (ignored line)", line, got)
		}
	}
}

func TestLintLineOutOfRange(t *testing.T) {
	findings := LintLine("61 5 * * * echo hi", 5)
	f := findAt(findings, 1)
	if f == nil {
		t.Fatalf("expected a finding at column 1, got %v", findings)
	}
	if f.Severity != SeverityError {
		t.Errorf("severity = %v, want error", f.Severity)
	}
	if f.EndColumn != 3 {
		t.Errorf("EndColumn = %d, want 3 (span of %q)", f.EndColumn, "61")
	}
}

func TestLintLineZeroStep(t *testing.T) {
	findings := LintLine("*/0 * * * * echo hi", 6)
	f := findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3, got %v", findings)
	}
	if f.Severity != SeverityError {
		t.Errorf("severity = %v, want error", f.Severity)
	}
}

func TestLintLineBackwardsRange(t *testing.T) {
	findings := LintLine("30 9 5-1 * * echo hi", 7)
	f := findAt(findings, 6)
	if f == nil {
		t.Fatalf("expected a finding at column 6, got %v", findings)
	}
	if f.EndColumn != 9 {
		t.Errorf("EndColumn = %d, want 9 (span of %q)", f.EndColumn, "5-1")
	}
}

func TestLintLineStepLargerThanMax(t *testing.T) {
	findings := LintLine("*/100 * * * * echo hi", 1)
	f := findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3, got %v", findings)
	}
	if f.Severity != SeverityWarning {
		t.Errorf("severity = %v, want warning", f.Severity)
	}
}

func TestLintLineMissingStepValue(t *testing.T) {
	findings := LintLine("5/ * * * * echo hi", 1)
	f := findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3, got %v", findings)
	}
}

func TestLintLineEmptyCommaItem(t *testing.T) {
	findings := LintLine("1,,3 * * * * echo hi", 1)
	f := findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3 (empty item between commas), got %v", findings)
	}
}

func TestLintLineNotEnoughFields(t *testing.T) {
	findings := LintLine("* * echo", 1)
	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1: %v", len(findings), findings)
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("severity = %v, want error", findings[0].Severity)
	}
}

func TestLintLineInvalidName(t *testing.T) {
	findings := LintLine("0 0 1 FOO * echo hi", 1)
	f := findAt(findings, 7)
	if f == nil {
		t.Fatalf("expected a finding at column 7, got %v", findings)
	}
}

func TestLintLineRestrictedDayFieldsWarning(t *testing.T) {
	findings := LintLine("0 9 15 * mon echo hi", 1)
	f := findAt(findings, 5)
	if f == nil {
		t.Fatalf("expected a finding at column 5, got %v", findings)
	}
	if f.Severity != SeverityWarning {
		t.Errorf("severity = %v, want warning", f.Severity)
	}
}

func TestLintLineRestrictedDayFieldsNoWarningWithStar(t *testing.T) {
	cases := []string{
		"0 9 15 * * echo hi",
		"0 9 * * mon echo hi",
	}
	for _, line := range cases {
		findings := LintLine(line, 1)
		if len(findings) != 0 {
			t.Errorf("LintLine(%q) = %v, want no findings", line, findings)
		}
	}
}

func TestLintLineSpecialSchedules(t *testing.T) {
	valid := []string{
		"@daily /usr/local/bin/backup.sh",
		"@reboot echo hi",
	}
	for _, line := range valid {
		if got := LintLine(line, 1); got != nil {
			t.Errorf("LintLine(%q) = %v, want no findings", line, got)
		}
	}

	findings := LintLine("@fortnightly echo hi", 1)
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Errorf("LintLine(@fortnightly ...) = %v, want one error", findings)
	}

	findings = LintLine("@daily", 1)
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Errorf("LintLine(@daily) = %v, want one error (missing command)", findings)
	}
}

// TestColumnMath checks that reported columns account for leading
// whitespace and multi-field offsets, not just position within a field's
// own text.
func TestColumnMath(t *testing.T) {
	// "  61 ..." - the bad value starts at column 3, after two leading spaces.
	findings := LintLine("  61 5 * * * echo hi", 1)
	f := findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3 (after leading whitespace), got %v", findings)
	}
	if f.EndColumn != 5 {
		t.Errorf("EndColumn = %d, want 5", f.EndColumn)
	}

	// second value in a comma list: "1,99" - 99 starts at offset 2 within
	// the field, and the field itself starts at column 1.
	findings = LintLine("1,99 * * * * echo hi", 1)
	f = findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3 (second comma item), got %v", findings)
	}
	if f.EndColumn != 5 {
		t.Errorf("EndColumn = %d, want 5", f.EndColumn)
	}

	// a value in a later field: hour field starts after "61 " -> column 4.
	findings = LintLine("0 25 * * * echo hi", 1)
	f = findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3 (hour field), got %v", findings)
	}
	if f.EndColumn != 5 {
		t.Errorf("EndColumn = %d, want 5", f.EndColumn)
	}

	// range with a bad hi bound: "5-99", hi starts right after "5-".
	findings = LintLine("5-99 * * * * echo hi", 1)
	f = findAt(findings, 3)
	if f == nil {
		t.Fatalf("expected a finding at column 3 (range hi bound), got %v", findings)
	}
	if f.EndColumn != 5 {
		t.Errorf("EndColumn = %d, want 5", f.EndColumn)
	}
}

func TestTokenizeColumns(t *testing.T) {
	toks := tokenize("  ab  cd\tef")
	want := []struct {
		text string
		col  int
	}{
		{"ab", 3}, {"cd", 7}, {"ef", 10},
	}
	if len(toks) != len(want) {
		t.Fatalf("tokenize() = %v, want %d tokens", toks, len(want))
	}
	for i, w := range want {
		if toks[i].text != w.text || toks[i].col != w.col {
			t.Errorf("toks[%d] = %+v, want text=%q col=%d", i, toks[i], w.text, w.col)
		}
	}
}

func TestIsEnvAssignment(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"MAILTO=root", true},
		{"PATH=/usr/bin:/bin", true},
		{"_FOO=bar", true},
		{"FOO2=bar", true},
		{"2FOO=bar", false},
		{"=novar", false},
		{"no equals here", false},
		{"FOO BAR=baz", false},
	}
	for _, c := range cases {
		if got := isEnvAssignment(c.in); got != c.want {
			t.Errorf("isEnvAssignment(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
