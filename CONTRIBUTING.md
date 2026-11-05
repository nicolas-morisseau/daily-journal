# Contributing

Bug reports, ideas and pull requests are all welcome. This file covers the
mechanics; [README.md](README.md#development) covers what the packages do.

## Before you start

For anything more than a fix, open an issue first. It costs a few minutes and
avoids the case where a finished pull request goes nowhere because the feature
does not belong in the tool. `journal` stays small on purpose: plain markdown
files, eight config keys, git for history and sync.

Good first contributions: a new date expression in `internal/dateparse`, a new
subcommand, a rough edge in the interactive views, anything in the README that
turned out to be wrong.

## Setting up

[mise](https://mise.jdx.dev) installs the toolchain and carries the handful of
commands used here. It is what CI installs from too, so the versions match.

```bash
git clone git@github.com:nicolas-morisseau/daily-journal.git
cd daily-journal
mise install             # the Go version in mise.toml, and GoReleaser
mise run build           # produces ./build/journal, and puts it on PATH
mise tasks               # everything else that is defined
```

mise is a convenience, not a requirement: the project is a plain Go module, and
`go build ./cmd/journal` works with any Go recent enough for the `go` directive
in `go.mod`. Only the release tooling really needs it.

Point the binary at a throwaway journal so your real entries stay out of it:

```bash
JOURNAL_DIR=/tmp/test-journal ./journal today
```

`XDG_CONFIG_HOME` does the same for the config file, which is handy when you
touch `internal/config`.

## The gate your pull request has to pass

Run this before pushing. CI runs this exact task — `pull-request.yml` installs
mise and calls it — so there is no second, hidden standard:

```bash
mise run check           # go vet, gofmt, go test
```

Without mise, the three commands it wraps:

```bash
go vet ./...
test -z "$(gofmt -l .)"
go test ./...
```

Then pick **one** of the `patch`, `minor` or `major` labels on the pull request:

| Label | For |
| --- | --- |
| `patch` | a fix, no change in behaviour |
| `minor` | a new command, option or behaviour |
| `major` | something that breaks an existing usage |

The label is a required check, and it is also what cuts the release: merging
tags the merge commit and publishes a version straight away. See
[docs/RELEASING.md](docs/RELEASING.md). If you have no permission to set labels,
say which one applies in the description and a maintainer will add it.

## What the code expects of a change

- **Tests come with the change.** Every package has them, including the
  interactive views — those inject keystrokes into the Bubble Tea models, so no
  terminal is involved. `internal/ui/render_manual_test.go` shows how to render
  a view for eyeballing:
  `CLICOLOR_FORCE=1 go test -run TestRenderPreview -v ./internal/ui`.
- **Printing and prompting stay out of the domain.** `internal/journal` returns
  data; `internal/ui` and `internal/cli` are what talk to the user.
- **Colours and styles live in `ui.Theme`,** never inline in a view.
- **A new command** implements `cli.Command` and joins `cli.Commands()`. Help
  and dispatch follow from that list — there is no second place to register it.
- **Config keys are forever.** Unknown keys are ignored rather than rejected so
  that an old config file keeps working; removing a key means it has to keep
  being ignored.
- **The entry format is a contract.** One markdown file per day, named
  `YYYY-MM-DD.md`. Anything that would make an existing journal unreadable is a
  `major` change and needs a very good reason.

Comments explain why, when the why is not obvious from the code. Match the
style of the file you are editing rather than introducing another one.

## Commits and pull requests

Commit subjects are lowercase, prefixed with the kind of change — `fix:`,
`feat:`, `docs:`, `refactor:`, `test:`, `chore:` — and written in the
imperative. Nothing enforces this; it just keeps `git log` readable. History is
not squashed for you, so tidy up your branch before asking for a review.

In the pull request, say what changed and why, and how you checked it. Screen
captures help for anything touching the views.

## Reporting a bug

Include your OS, the output of `journal --version`, and what you ran against
what you expected. `journal help` prints the resolved journal directory, config
path and editor — that block answers most questions about configuration. Strip
anything private out of it before pasting: journal entries are yours, and no
maintainer needs them to fix a bug.
