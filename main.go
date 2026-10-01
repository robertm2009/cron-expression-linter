package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main without the process-global bits so it can be tested.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cronlint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print findings as a single JSON array instead of text")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: cronlint [--json] <file> [file ...]")
		fmt.Fprintln(stderr, "       cronlint [--json] -            (read from stdin)")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fs.Usage()
		return 2
	}

	var collected []jsonFinding
	emit := func(file, source string, f Finding) {
		if *asJSON {
			collected = append(collected, newJSONFinding(file, f))
			return
		}
		printFinding(stdout, file, source, f)
	}

	exitCode := 0
	for _, path := range paths {
		hadIssues, err := lintFile(path, stdin, emit)
		if err != nil {
			fmt.Fprintf(stderr, "cronlint: %s: %v\n", path, err)
			exitCode = 2
			continue
		}
		if hadIssues && exitCode == 0 {
			exitCode = 1
		}
	}

	if *asJSON {
		if err := writeJSON(stdout, collected); err != nil {
			fmt.Fprintf(stderr, "cronlint: %v\n", err)
			return 2
		}
	}
	return exitCode
}

func lintFile(path string, stdin io.Reader, emit func(file, source string, f Finding)) (bool, error) {
	displayName := path
	var r io.Reader
	if path == "-" {
		displayName = "stdin"
		r = stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()
		r = f
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	hadErrors := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		for _, f := range LintLine(line, lineNo) {
			if f.Severity == SeverityError {
				hadErrors = true
			}
			emit(displayName, line, f)
		}
	}
	if err := scanner.Err(); err != nil {
		return hadErrors, err
	}
	return hadErrors, nil
}
