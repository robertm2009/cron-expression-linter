package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// printFinding renders a finding the way a compiler would: a one-line
// summary with file:line:column, followed by the offending source line and
// a caret underline pointing at the exact span that's wrong.
func printFinding(w io.Writer, filename, sourceLine string, f Finding) {
	fmt.Fprintf(w, "%s:%d:%d: %s: %s\n", filename, f.Line, f.Column, f.Severity, f.Message)

	gutter := fmt.Sprintf("%5d | ", f.Line)
	fmt.Fprintf(w, "%s%s\n", gutter, sourceLine)

	width := f.EndColumn - f.Column
	if width < 1 {
		width = 1
	}
	pad := strings.Repeat(" ", f.Column-1)
	marker := strings.Repeat("^", width)
	fmt.Fprintf(w, "%s%s%s\n", strings.Repeat(" ", len(gutter)), pad, marker)
}

// jsonFinding is the wire format for --json. Columns are 1-based and
// end_column is exclusive, matching Finding.
type jsonFinding struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndColumn int    `json:"end_column"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

func newJSONFinding(file string, f Finding) jsonFinding {
	return jsonFinding{
		File:      file,
		Line:      f.Line,
		Column:    f.Column,
		EndColumn: f.EndColumn,
		Severity:  f.Severity.String(),
		Message:   f.Message,
	}
}

// writeJSON emits one array covering every input file. An empty result is
// written as [] rather than null so consumers don't need a special case.
func writeJSON(w io.Writer, findings []jsonFinding) error {
	if findings == nil {
		findings = []jsonFinding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(findings)
}
