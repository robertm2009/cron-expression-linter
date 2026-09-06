# cronlint

A linter for crontab-style cron expressions. It reads a crontab file, checks
each schedule line against the five standard cron fields, and reports
problems the way a compiler would: file, line, column, and a caret pointing
at the exact text that's wrong.

## Why

Cron failures are quiet. A typo like `*/0` (a step of zero) or `5-1` (a
backwards range) doesn't error out when you install the crontab - it just
silently never fires, or fires in a way you didn't expect, and you find out
days later when the backup didn't run. Most advice for catching this is "look
at it carefully" or paste it into a web form. This is a small, dependency-free
tool you can run against a crontab file directly, or wire into a pre-commit
hook or CI step for a repo that manages cron schedules as text.

## Install

Requires Go 1.22+. No third-party dependencies.

```
go build -o cronlint .
```

## Usage

```
cronlint crontab.txt
cronlint file1 file2 file3
cat crontab.txt | cronlint -
```

Exit status is `0` if no errors were found, `1` if any line has an error
(warnings alone do not affect the exit status), and `2` on a usage or I/O
problem such as a missing file.

## Example

Given `crontab.txt`:

```
# run backup every night
0 3 * * * /usr/local/bin/backup.sh

# broken: minute out of range, zero step, backwards range
61 5 * * * echo hi
*/0 * * * * echo hi
30 9 5-1 * * echo hi
```

Running `cronlint crontab.txt` produces:

```
crontab.txt:5:1: error: minute value 61 is out of range (must be 0-59)
    5 | 61 5 * * * echo hi
        ^^
crontab.txt:6:3: error: step value must be a positive whole number, got 0
    6 | */0 * * * * echo hi
          ^
crontab.txt:7:6: error: invalid day-of-month range: start 5 is greater than end 1
    7 | 30 9 5-1 * * echo hi
             ^^^
```

The valid line (line 2) produces no output. Comments, blank lines, and
crontab variable assignments (`MAILTO=root`, `PATH=/usr/bin:/bin`, ...) are
skipped rather than treated as schedules.

## What it checks

For each of the five fields (minute, hour, day-of-month, month,
day-of-week), on every comma-separated item:

- Bare numbers and `*` are accepted; month and day-of-week also accept
  three-letter names (`JAN`-`DEC`, `SUN`-`SAT`, case-insensitive).
- Numbers and names must fall inside the field's valid range
  (e.g. hour is 0-23, day-of-month is 1-31).
- Ranges (`a-b`) must have a start that is not greater than the end.
- Step values (`.../n`) must be positive whole numbers. A step larger than
  the field's own maximum is flagged as a warning, since it can only ever
  match once.
- A schedule line must have at least the 5 fields before its command.

## Not yet supported

- `@daily`, `@reboot`, and the other `@`-shorthands.
- Non-standard extensions like `L`, `W`, or `#` in day fields (Quartz-style).
- Cross-field checks, such as day-of-month and day-of-week both being
  restricted (most cron implementations OR these together, which is a
  common source of confusion).

## License

MIT, see [LICENSE](LICENSE).
