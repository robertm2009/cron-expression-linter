package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Severity distinguishes findings that block a schedule from being usable
// (Error) from ones that are legal but probably not what the author meant
// (Warning).
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Finding is one problem reported at a specific line/column in the input.
// Column is 1-based and points at the first rune of the offending text;
// EndColumn is exclusive, so EndColumn-Column gives the span to underline.
type Finding struct {
	Line      int
	Column    int
	EndColumn int
	Severity  Severity
	Message   string
}

type fieldSpec struct {
	name  string
	min   int
	max   int
	names map[string]int // nil for purely numeric fields
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

// scheduleFields is the standard five-field vixie-cron layout. Seventh-day
// aliasing (0 and 7 both meaning Sunday) is handled by day-of-week's range.
var scheduleFields = []fieldSpec{
	{name: "minute", min: 0, max: 59},
	{name: "hour", min: 0, max: 23},
	{name: "day-of-month", min: 1, max: 31},
	{name: "month", min: 1, max: 12, names: monthNames},
	{name: "day-of-week", min: 0, max: 7, names: dowNames},
}

type token struct {
	text string
	col  int // 1-based column of the token's first rune
}

func tokenize(line string) []token {
	runes := []rune(line)
	n := len(runes)
	var toks []token
	col := 1
	i := 0
	for i < n {
		for i < n && isSpace(runes[i]) {
			i++
			col++
		}
		if i >= n {
			break
		}
		start, startCol := i, col
		for i < n && !isSpace(runes[i]) {
			i++
			col++
		}
		toks = append(toks, token{text: string(runes[start:i]), col: startCol})
	}
	return toks
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isEnvAssignment recognizes crontab variable lines such as "MAILTO=root" or
// "PATH=/usr/bin:/bin", which are not schedule lines at all and should not
// be linted as one.
func isEnvAssignment(trimmed string) bool {
	eq := strings.IndexByte(trimmed, '=')
	if eq <= 0 {
		return false
	}
	name := trimmed[:eq]
	if strings.ContainsAny(name, " \t") {
		return false
	}
	for i, r := range name {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

// specialSchedules are the vixie-cron @-shorthands that stand in for the
// five time fields entirely. They are matched exactly as written here;
// real cron does not case-fold them, so neither do we.
var specialSchedules = map[string]bool{
	"@reboot":   true,
	"@yearly":   true,
	"@annually": true,
	"@monthly":  true,
	"@weekly":   true,
	"@daily":    true,
	"@midnight": true,
	"@hourly":   true,
}

// lintSpecialSchedule handles a line whose first token starts with '@',
// which replaces the five time fields rather than being one of them.
func lintSpecialSchedule(toks []token, lineNo int) []Finding {
	tok := toks[0]
	endCol := tok.col + len([]rune(tok.text))
	if !specialSchedules[tok.text] {
		return []Finding{errf(lineNo, tok.col, endCol,
			"unrecognized schedule shorthand %q (expected one of @reboot, @yearly, @annually, @monthly, @weekly, @daily, @midnight, @hourly)",
			tok.text)}
	}
	if len(toks) < 2 {
		return []Finding{errf(lineNo, endCol, endCol+1,
			"missing command after %s", tok.text)}
	}
	return nil
}

// LintLine checks a single line of a crontab file and returns any findings.
// lineNo is 1-based to match how editors and compilers report locations.
func LintLine(line string, lineNo int) []Finding {
	line = strings.TrimSuffix(line, "\r")
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || isEnvAssignment(trimmed) {
		return nil
	}

	toks := tokenize(line)
	if strings.HasPrefix(toks[0].text, "@") {
		return lintSpecialSchedule(toks, lineNo)
	}
	if len(toks) < 5 {
		last := toks[len(toks)-1]
		endCol := last.col + len([]rune(last.text))
		return []Finding{errf(lineNo, endCol, endCol+1,
			"expected 5 schedule fields (minute hour day-of-month month day-of-week) before the command, found only %d",
			len(toks))}
	}

	var findings []Finding
	for i, spec := range scheduleFields {
		findings = append(findings, validateField(toks[i], spec, lineNo)...)
	}
	findings = append(findings, checkRestrictedDayFields(toks[2], toks[4], lineNo)...)
	return findings
}

// checkRestrictedDayFields warns about the classic cron gotcha: when both
// day-of-month and day-of-week are restricted (anything other than "*"),
// vixie-cron and most of its descendants OR the two fields together instead
// of ANDing them, so the job runs on a match against either one. Someone
// expecting "the 15th, but only if it's a Monday" instead gets "the 15th, or
// any Monday".
func checkRestrictedDayFields(dom, dow token, lineNo int) []Finding {
	if dom.text == "*" || dow.text == "*" {
		return nil
	}
	endCol := dom.col + len([]rune(dom.text))
	return []Finding{warnf(lineNo, dom.col, endCol,
		"day-of-month (%q) and day-of-week (%q) are both restricted; most cron implementations OR these fields together, so the job runs when either matches rather than only when both do",
		dom.text, dow.text)}
}

type part struct {
	text   string
	offset int // rune offset of this part within its parent field text
}

func splitWithOffsets(text string, sep rune) []part {
	runes := []rune(text)
	var parts []part
	start := 0
	for i, r := range runes {
		if r == sep {
			parts = append(parts, part{text: string(runes[start:i]), offset: start})
			start = i + 1
		}
	}
	parts = append(parts, part{text: string(runes[start:]), offset: start})
	return parts
}

func validateField(tok token, spec fieldSpec, lineNo int) []Finding {
	var findings []Finding
	for _, p := range splitWithOffsets(tok.text, ',') {
		findings = append(findings, validateItem(p, tok.col, spec, lineNo)...)
	}
	return findings
}

func validateItem(p part, fieldCol int, spec fieldSpec, lineNo int) []Finding {
	if p.text == "" {
		col := fieldCol + p.offset
		return []Finding{errf(lineNo, col, col+1,
			"empty value in %s field (check for a stray or trailing comma)", spec.name)}
	}

	base := p.text
	stepText := ""
	hasStep := false
	stepOffset := 0
	if idx := strings.IndexRune(p.text, '/'); idx >= 0 {
		base = p.text[:idx]
		stepText = p.text[idx+1:]
		hasStep = true
		stepOffset = p.offset + idx + 1
	}

	var findings []Finding
	baseCol := fieldCol + p.offset
	baseLen := len([]rune(base))

	switch {
	case base == "*":
		// matches every value in range; nothing to check
	case strings.ContainsRune(base, '-'):
		bounds := strings.SplitN(base, "-", 2)
		loText, hiText := bounds[0], bounds[1]
		hiCol := baseCol + len([]rune(loText)) + 1

		loVal, loOK := checkValue(&findings, spec, loText, lineNo, baseCol)
		hiVal, hiOK := checkValue(&findings, spec, hiText, lineNo, hiCol)

		if loOK && hiOK && loVal > hiVal {
			findings = append(findings, errf(lineNo, baseCol, baseCol+baseLen,
				"invalid %s range: start %d is greater than end %d", spec.name, loVal, hiVal))
		}
	default:
		checkValue(&findings, spec, base, lineNo, baseCol)
	}

	if hasStep {
		stepCol := fieldCol + stepOffset
		stepLen := len([]rune(stepText))
		if stepText == "" {
			findings = append(findings, errf(lineNo, stepCol, stepCol+1,
				"missing step value after '/'"))
		} else if stepVal, err := strconv.Atoi(stepText); err != nil {
			findings = append(findings, errf(lineNo, stepCol, stepCol+stepLen,
				"invalid step value %q: must be a positive whole number", stepText))
		} else if stepVal <= 0 {
			findings = append(findings, errf(lineNo, stepCol, stepCol+stepLen,
				"step value must be a positive whole number, got %d", stepVal))
		} else if stepVal > spec.max {
			findings = append(findings, warnf(lineNo, stepCol, stepCol+stepLen,
				"step value %d is larger than the field's maximum (%d); it will only ever match once", stepVal, spec.max))
		}
	}

	return findings
}

// checkValue parses and range-checks a single value, appending any finding
// to findings. It returns the parsed value and whether it was fully valid,
// so callers can decide whether it is safe to compare against another value
// (e.g. for range order checks).
func checkValue(findings *[]Finding, spec fieldSpec, text string, lineNo, col int) (int, bool) {
	length := len([]rune(text))
	val, ok := parseFieldValue(spec, text)
	if !ok {
		msg := "not a number"
		if spec.names != nil {
			msg = "not a number and not a recognized name"
		}
		*findings = append(*findings, errf(lineNo, col, col+length,
			"invalid %s value %q: %s", spec.name, text, msg))
		return 0, false
	}
	if val < spec.min || val > spec.max {
		*findings = append(*findings, errf(lineNo, col, col+length,
			"%s value %d is out of range (must be %d-%d)", spec.name, val, spec.min, spec.max))
		return val, false
	}
	return val, true
}

func parseFieldValue(spec fieldSpec, text string) (int, bool) {
	if spec.names != nil {
		if v, ok := spec.names[strings.ToUpper(text)]; ok {
			return v, true
		}
	}
	v, err := strconv.Atoi(text)
	if err != nil {
		return 0, false
	}
	return v, true
}

func errf(line, col, endCol int, format string, args ...interface{}) Finding {
	return Finding{Line: line, Column: col, EndColumn: endCol, Severity: SeverityError, Message: fmt.Sprintf(format, args...)}
}

func warnf(line, col, endCol int, format string, args ...interface{}) Finding {
	return Finding{Line: line, Column: col, EndColumn: endCol, Severity: SeverityWarning, Message: fmt.Sprintf(format, args...)}
}
