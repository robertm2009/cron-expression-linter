package main

import (
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
