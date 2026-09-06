package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: cronlint <file> [file ...]")
		fmt.Fprintln(os.Stderr, "       cronlint -            (read from stdin)")
		os.Exit(2)
	}

	exitCode := 0
	for _, path := range args {
		hadIssues, err := lintFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cronlint: %s: %v\n", path, err)
			exitCode = 2
			continue
		}
		if hadIssues && exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func lintFile(path string) (bool, error) {
	displayName := path
	var r io.Reader
	if path == "-" {
		displayName = "stdin"
		r = os.Stdin
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
			printFinding(os.Stdout, displayName, line, f)
		}
	}
	if err := scanner.Err(); err != nil {
		return hadErrors, err
	}
	return hadErrors, nil
}
