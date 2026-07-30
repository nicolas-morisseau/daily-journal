<div align="center">

# daily-journal

**A journal you keep from the terminal.**
One markdown file per day, in a git repository you own.

[![Release](https://img.shields.io/github/v/release/nicolas-morisseau/daily-journal)](https://github.com/nicolas-morisseau/daily-journal/releases/latest)
[![CI](https://github.com/nicolas-morisseau/daily-journal/actions/workflows/pull-request.yml/badge.svg)](https://github.com/nicolas-morisseau/daily-journal/actions/workflows/pull-request.yml)
[![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

```
╭──────────────────────────────╮ ╭──────────────────────────────────────────╮ ╭──────────────────────────╮
│          July 2026           │ │ Tuesday, 28 July 2026           58 words │ │ THIS MONTH               │
│                              │ │ ──────────────────────────────────────── │ │ Entries               19 │
│  Mo  Tu  We  Th  Fr  Sa  Su  │ │ 2026-07-28                               │ │ Words              3 051 │
│           1   2   3   4   5  │ │                                          │ │ Days               19/31 │
│   6   7   8   9  10  11  12  │ │ 09:12 - Finally understood why the       │ │                          │
│  13  14  15  16  17  18  19  │ │ deploy was flaky: the cache warmup       │ │ STREAKS                  │
│  20  21  22  23  24  25  26  │ │ raced the health check. Wrote it up      │ │ Current           2 days │
│  27  28  29  30  31          │ │ for the team. [#work]                    │ │ Longest           5 days │
│                              │ │                                          │ │                          │
│ ● today + entry              │ │ 14:30 - Long walk, no phone. The best    │ │ THIS WEEK                │
│ ● entry                      │ │ ideas still show up off-screen. [#life]  │ │ Mo ████████████ 159      │
│ ● today                      │ │                                          │ │ Tu ████ 58               │
│                              │ │                                          │ │ We                       │
│ NAVIGATE                     │ │                                          │ │ Th                       │
│  ← →  day        t  today    │ │                                          │ │ Fr                       │
│  ↑ ↓  week       e  edit     │ │                                          │ │ Sa                       │
│  n p  month      q  quit     │ │                                          │ │ Su                       │
│                              │ │                                          │ │                          │
│                              │ │                                          │ │ TOP TAGS                 │
│                              │ │                                          │ │ #work      ▪▪▪▪▪▪ 10     │
│                              │ │                                          │ │ #life      ▪▪▪▪ 7        │
╰──────────────────────────────╯ ╰──────────────────────────────────────────╯ ╰──────────────────────────╯
```

The whole daily loop is two commands:

```bash
journal                              # open today's entry in $EDITOR
journal "Had a breakthrough today"   # capture a line without opening anything
```

No account, no database, nothing to sync. Your entries are plain markdown files
you could still read with `cat` in ten years, and git covers history, backup and
sync — because the journal *is* a git repository.

## Features

- **Quick capture** — `journal "a thought"` appends a timestamped line to
  today's entry and returns. Nothing to open, nothing to close.
- **Your editor** — entries open in `$EDITOR`. There is no built-in editor to
  learn.
- **A calendar to read back** — full-screen month view with the selected day's
  markdown rendered next to it, plus streaks, word counts and your most used
  tags. Press `e` on any day to edit it in place.
- **A stand-up view** — `journal daily` puts yesterday and today's notes next to
  the commits you pushed in the same window, and takes a new note straight from
  the keyboard. What you need to talk for two minutes, on one screen.
- **Search** — case-insensitive full-text search with context lines, optionally
  narrowed to entries carrying `#tags`.
- **A sprint review** — `journal review` lines the notes you tagged `#good`,
  `#bad` and `#mad` over the last two weeks up in three columns, dated, ready to
  walk through in a retro.
- **Natural dates** — `journal open yesterday`, `"last friday"`, `"3 days ago"`,
  `2025-01-15`. Same grammar on macOS and Linux.
- **Git built in** — pulls before you write, offers to commit and push after
  (or does it silently with `AUTOSYNC=true`).
- **Plain files** — one `YYYY-MM-DD.md` per day in a directory you choose.
  No lock-in; leaving means keeping your files.

## Installation

### Homebrew

```bash
brew tap nicolas-morisseau/tap
brew install daily-journal
```

The formula lives in
[nicolas-morisseau/homebrew-tap](https://github.com/nicolas-morisseau/homebrew-tap),
which is what `nicolas-morisseau/tap` resolves to.

The formula installs a prebuilt binary (macOS and Linux, amd64 and arm64) rather
than compiling one, so Go is not needed. A formula and not a cask on purpose:
Homebrew quarantines cask downloads, and macOS kills a quarantined binary that
carries only the ad-hoc signature the Go linker applies. Casks are also macOS
only.

> [!NOTE]
> [journalot](https://github.com/jtaylortech/journalot) provides a `journal`
> command too, so Homebrew refuses to install both. Run
> `brew uninstall journalot && brew untap jtaylortech/journalot` first.
> daily-journal reads the same journal.

### Prebuilt binaries

Every [release](https://github.com/nicolas-morisseau/daily-journal/releases)
carries tarballs and a `checksums.txt`:

```bash
V=0.1.0; OS=darwin; ARCH=arm64   # or linux, amd64
curl -fsSL "https://github.com/nicolas-morisseau/daily-journal/releases/download/v$V/daily-journal_${V}_${OS}_${ARCH}.tar.gz" \
  | tar xz journal && mv journal ~/.local/bin/
```

This is also the way to install a version that is not the latest: binaries stay
on GitHub for every release.

### From source

Requires Go 1.26 or newer:

```bash
git clone https://github.com/nicolas-morisseau/daily-journal
cd daily-journal
go build -o ~/.local/bin/journal ./cmd/journal
```

The command is `journal`, not `daily-journal`: typing three syllables to jot a
thought defeats the point. git is the only runtime requirement.

## Usage

```
Usage: journal [COMMAND] [OPTIONS]
       journal "text to capture"
```

| Command | What it does |
| --- | --- |
| `journal` | Open today's entry in your editor |
| `journal "some text"` | Append a timestamped line to today, no editor |
| `journal open [DATE]` | Open a specific day (`yesterday`, `"last friday"`, `2025-01-15`) |
| `journal open --last` | Open the most recent entry |
| `journal add [--date DATE] TEXT...` | Capture into a given day |
| `journal search [--tag TAG]... QUERY...` | Search the text, optionally within tags |
| `journal week` | Open every entry from the current week at once |
| `journal calendar [YYYY-MM]` | Full-screen month view: read and edit entries |
| `journal daily [--days N]` | Stand-up view: the last day's notes and git commits |
| `journal review [--weeks N] [--from DATE] [--to DATE]` | Sprint review: `#good`, `#bad` and `#mad` side by side |
| `journal version` | Print the version of this build |
| `journal help [COMMAND]` | Usage for the tool or for one command |

### Quick capture vs. commands

The first argument decides: an exact command name runs that command, anything
else is text to capture. A quoted note is a single argument, so
`journal "list of things to buy"` can never be mistaken for a command — only a
bare single word that *is* a command name collides, and then it fails loudly
rather than guessing. Two escape hatches:

```bash
journal add search for a new flat   # explicit
journal -- -5 degrees this morning  # POSIX separator, also for leading dashes
```

### Searching

`--tag` narrows a text search to entries carrying a tag; repeating it narrows
further (tags combine with **and**):

```bash
journal search roadmap                    # every entry mentioning it
journal search --tag work roadmap         # only within #work entries
journal search --tag work --tag bad why   # entries carrying both tags
journal search --tag work,bad why         # same, comma-separated
```

Tag matching is exact on the word — `--tag work` does not pick up `#workshop`,
and a tag written `[#work]` still counts. The summary line reports how many
entries were searched, so an empty result is explainable.

### The calendar

`journal calendar` is the main way to read back: month grid on the left, the
selected day's markdown in the middle, stats on the right. Moving on the grid
reloads the middle panel; `e` hands the terminal to `$EDITOR` on that day's
entry — creating it if needed — and picks the view back up when you quit.

| Key | | Key | |
| --- | --- | --- | --- |
| `←` `→` | day | `j` `k` `^d` `^u` `g` `G` | scroll the entry |
| `↑` `↓` | week | `e` / `enter` | edit the day |
| `n` `p` | month | `r` | sprint review of that day |
| `t` | today | `q` | quit |

`r` switches to the [sprint review](#the-sprint-review) of the period the selected
day belongs to, and `q` there comes straight back to the month you left — same
day, reloaded in case you edited an entry on the way.

Side panels drop out on narrow terminals so the markdown always keeps room.
Piped, `journal calendar` prints a plain month grid instead, so it stays usable
in scripts.

### The daily stand-up

`journal daily` is the screen to open two minutes before the stand-up: what you
wrote yesterday and today on the left, what you committed over the same window on
the right, and a box at the bottom to add a line without leaving.

```
  DAILY  Tuesday 28 July 2026                                 3 notes  ·  39 words  ·  3 commits
  Since yesterday  ·  Mon 27 Jul → Tue 28 Jul

  ╭────────────────────────────────────────────────╮ ╭────────────────────────────────────────────╮
  │ JOURNAL                                3 notes │ │ GIT                   3 commits in 2 repos │
  │ ────────────────────────────────────────────── │ │ ────────────────────────────────────────── │
  │ Today · Tuesday 28 July                        │ │ stats-service                            2 │
  │ │ 09:12  Taking the logback bump through the   │ │ │ 676ee18 chore(log): bump the… 3 hours ago │
  │ │        last four services.                   │ │ │ 468a5d8 fix: the classpath    5 hours ago │
  │                                                │ │                                            │
  │ │ 11:40  The classpath error was a missing     │ │ mailer-service                           1 │
  │ │        protobuf schema, not the Scala 3      │ │ │ 9329a3a feat: migrate the reg…  yesterday │
  │ │        move.                                 │ │                                            │
  │                                                │ │                                            │
  │ Yesterday · Monday 27 July                     │ │                                            │
  │ │ 17:05  Shipped the GitHub Packages switch    │ │                                            │
  │ │        on stats-service.                     │ │                                            │
  ╰────────────────────────────────────────────────╯ ╰────────────────────────────────────────────╯

  ╭────────────────────────────────────────────────────────────────────────────────────────────────╮
  │ › Add a line to today's entry…                                                                 │
  ╰────────────────────────────────────────────────────────────────────────────────────────────────╯
  i type a note  ·  e edit today  ·  tab pane  ·  j k scroll  ·  r refresh  ·  q quit
```

The journal side shows every note of the window, timestamped or not, days without
an entry included — a stand-up that hid them would be lying about the day. Tags
stay as written. Times sit in a column of their own so the day is scanned down the
left edge rather than read through; on a pane too narrow for that column, they take
the line above their note instead.

| Key | | Key | |
| --- | --- | --- | --- |
| `i` / `a` | type a note | `e` / `enter` | open today's entry in `$EDITOR` |
| `enter` | add it to today | `tab` | switch pane |
| `esc` | stop typing | `j` `k` `^d` `^u` `g` `G` | scroll the focused pane |
| `ctrl+c` | quit while typing | `r` | re-read both halves |
| | | `q` | quit |

Typing is a mode, so the shortcuts stay short: `i` opens the box, `enter` writes
the line into today's entry exactly as `journal "a thought"` would and leaves the
box open for the next one, `esc` gives the keyboard back. What you typed survives
a failure, and the new note shows up in the pane straight away.

The window is a day back by default, the same as `git standup -d 1`. On a Monday,
widen it:

```bash
journal daily --days 3     # Friday, the weekend and today
```

#### The git half

The commits come from [git-standup](https://github.com/kamranahmedse/git-standup),
which walks a directory of repositories and lists what you committed in each. Two
things to have:

```bash
brew install git-standup

# In ~/.config/daily-journal/config
PROJECTS_DIR="$HOME/projects"
```

`journal daily` then runs `git standup -s -d <days>` there and groups the result
by repository. It reads the current branch of every repository under
`PROJECTS_DIR`, filtered to your own commits — that is git-standup's own default,
not something this tool decides.

Neither is required. Without the helper, or without `PROJECTS_DIR`, the git pane
says which one is missing and the journal half works exactly as before. The walk
takes a few seconds over a hundred repositories, so it runs in the background: the
pane fills in when it is done, and the rest of the view never waits for it.

Below two columns' worth of width the view shows one pane at a time, and `tab`
walks between them. Piped, `journal daily` prints both halves as plain text, ready
to paste into wherever the stand-up gets written down:

```bash
journal daily | pbcopy
```

### The sprint review

`journal review` pulls the notes you tagged `#good`, `#bad` and `#mad` out of the
last two weeks and puts them side by side, one column per mood, each note
carrying the day it was written:

```
  SPRINT REVIEW  15 Jul → 28 Jul 2026              14 days  ·  9 entries  ·  6 notes

  ╭──────────────────────────────╮ ╭──────────────────────────────╮ ╭──────────────────────────────╮
  │ ✓ GOOD                   1/3 │ │ ▲ BAD                      2 │ │ ✖ MAD                      1 │
  │ ──────────────────────────── │ │ ──────────────────────────── │ │ ──────────────────────────── │
  │ ▌ Mon 20 Jul  09:12          │ │ │ Mon 20 Jul  11:49          │ │ │ Thu 23 Jul  16:40          │
  │ ▌ Finally understood why the │ │ │ Lost in the MRs. sbt 2     │ │ │ The staging database was   │
  │ ▌ deploy was flaky: the      │ │ │ broke the double push, and │ │ │ wiped again by a job       │
  │ ▌ cache warmup raced the     │ │ │ fixing it took the whole   │ │ │ nobody owns.               │
  │ ▌ health check.              │ │ │ morning.                   │ │                              │
  │                              │ │                              │ │                              │
  │ │ Thu 23 Jul  18:16          │ │ │ Mon 27 Jul  11:35          │ │                              │
  │ │ All hands. Good to have    │ │ │ Found a public repo with   │ │                              │
  │ │ the swarms' objectives in  │ │ │ the internal roadmap in    │ │                              │
  │ │ one place.                 │ │ │ it. Back to internal.      │ │                              │
  ╰──────────────────────────────╯ ╰──────────────────────────────╯ ╰──────────────────────────────╯

  ← → mood  ·  ↑ ↓ note  ·  e edit  ·  p n period  ·  t now  ·  q quit
```

A note is one timestamped line — what `journal "a thought"` writes — with
everything that follows it until the next one, so a note edited into a paragraph
stays one card. A tagged line written by hand, bullet or not, counts too. The
mood tag labels the column, so the cards drop it; your other tags stay.

| Key | | Key | |
| --- | --- | --- | --- |
| `←` `→` | mood column | `e` / `enter` | edit the note's day |
| `↑` `↓` `^d` `^u` `g` `G` | note | `p` `n` | previous / next period |
| | | `t` | back to the starting period |
| | | `q` | quit |

Paging moves by the window's own length: on the default fortnight, `p` twice
lands a month back. The period itself is yours to choose:

```bash
journal review                          # the last two weeks
journal review --weeks 3                # a longer sprint
journal review --to "last friday"       # the window ending then
journal review --from 2026-07-01        # everything since, whatever its length
```

#### Real sprints, not the last fourteen days

By default the window rolls: open it on a Tuesday and it runs Tuesday to Tuesday.
If your sprints have actual boundaries, name the first day of any one of them
once and the window snaps to them instead:

```bash
# In ~/.config/daily-journal/config
SPRINT_START=2026-07-06
```

From that Monday, sprints run 6–19 July, 20 July–2 August, and so on. Now
`journal review` shows the sprint you are *in*, whichever day you open it — the
same bounds on the Monday and on the Friday of week two — and `p` walks back a
whole sprint at a time. The anchor is just a reference point: an earlier date
works, and so does a future one, since the boundaries are counted in both
directions.

`--weeks` is the sprint length, so it also decides where the boundaries fall.
`--from` still means exactly what it says and ignores the anchor. With no
`SPRINT_START`, nothing changes: the window keeps rolling with today.

Below three columns' worth of width the view shows one mood at a time, and `←`
`→` walk between them. Piped, `journal review` prints the same columns as plain
text — enough to paste into a retro doc.

`journal calendar` reaches the same view: press `r` on any day for the review of
the period it belongs to, and `q` to go back to the month.

## Configuration

Everything works with no configuration at all. To change something, create
`~/.config/daily-journal/config`:

```bash
# Where the journal lives. Default: ~/daily-journal
JOURNAL_DIR="$HOME/journal"

# Commit and push on save without asking. Default: off
AUTOSYNC=true

# Branch used for pull and push. Default: main
GIT_BRANCH=main

# Editor to open entries with. Default: $EDITOR, then code > vim > nano
EDITOR="hx --vsplit"

# First day of a sprint, for `journal review`. Default: unset, rolling window
SPRINT_START=2026-07-06

# Repositories `journal daily` reads git activity from. Default: unset, no git half
PROJECTS_DIR="$HOME/projects"
```

Those six keys are all there is.

| Key | Default | Notes |
| --- | --- | --- |
| `JOURNAL_DIR` | `~/daily-journal` | `~` and `$HOME` are expanded. Created on first write. |
| `AUTOSYNC` | off | Only the exact string `true` enables it — `1` and `yes` do not. |
| `GIT_BRANCH` | `main` | Used by `git pull` and `git push`. |
| `EDITOR` | `$EDITOR` | Quote it to pass arguments: `EDITOR="code --wait"`. |
| `SPRINT_START` | unset | One `YYYY-MM-DD` date. Only `journal review` reads it, and only it fails on a bad value. |
| `PROJECTS_DIR` | unset | Directory holding your repositories. Only `journal daily` reads it, and it says so in place rather than failing. |

<details>
<summary><strong>File format details</strong></summary>

The file is **read, not executed**, but it understands the shell syntax you
would expect from the days when it was `source`d:

- comments, on their own line or trailing;
- an optional `export` prefix;
- **double quotes and bare words expand** `$VAR` and `${VAR}`, from the
  environment or from a key assigned earlier in the same file;
- **single quotes are literal** — `JOURNAL_DIR='$HOME/x'` names a directory
  literally called `$HOME/x`;
- **a bare word ends at the first space** — `EDITOR=hx --vsplit` sets `hx` and
  drops the rest; quote it;
- **an empty assignment counts as unset**, falling back to the default;
- **unknown keys are ignored**, so a config written for an older version keeps
  working (`MULTI_JOURNAL`, `ACTIVE_JOURNAL`, `DISABLE_PROMPTS` are skipped).

`JOURNAL_DIR`, `PROJECTS_DIR` and `EDITOR` are also read from the environment, and
`XDG_CONFIG_HOME` moves the config directory. The config file wins over the
environment.

The file is never written by the tool. It is not created on first run, and
`journal help` prints its path whether or not it exists. The one exception is
migration: a config left by an earlier version (`~/.config/journalot/config`,
then `~/.journalotrc`) is moved into place on first run, and the move is
reported. An existing `~/.config/daily-journal/config` always wins.

</details>

### Entry template

Drop a `template.md` in the journal directory and new entries start from it
instead of a bare date heading:

```markdown
# {{date}} — {{day_of_week}} (week {{week_number}})

Started at {{time}}

## Notes
```

`{{date}}` is the entry's date; the other placeholders describe the moment of
creation.

## How your data is stored

```
~/daily-journal/             default location; JOURNAL_DIR overrides it
├── entries/
│   ├── 2026-07-28.md        one file per day
│   └── 2026-07-27.md
└── template.md              optional
```

The journal is a git repository, and that is what covers history, backup and
sync — there is no export and no backup command, on purpose:

```bash
cd ~/daily-journal
git init
git remote add origin git@github.com:<you>/my-private-journal.git
git push -u origin main
```

The first time `journal` runs in a directory that is not a repository, it
offers to initialise one. On open it pulls from `origin`; on save it commits
and pushes — asking first unless `AUTOSYNC=true`, and only when the editor
actually changed the file.

A journal left in `~/journalot` by an earlier version keeps being used as long
as `~/daily-journal` does not exist, so nothing has to be moved. Rename it, or
set `JOURNAL_DIR`, whenever you prefer.

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | The command ran and failed (empty journal, git error, unreadable file) |
| 2 | The command line was wrong (unknown option, missing argument) |

## Development

The Go toolchain is the task runner — there is no build system to learn:

```bash
go build ./cmd/journal   # produces ./journal
go test ./...            # full suite
go vet ./... && test -z "$(gofmt -l .)"   # the pre-commit gate

# render the TUI views without a terminal, to eyeball a layout change
CLICOLOR_FORCE=1 go test -run TestRenderPreview -v ./internal/ui
CLICOLOR_FORCE=1 go test -run 'TestRender(Review|Daily)Preview' -v ./internal/ui
```

```
.github/            CI: the pull request gate, tagging, releasing (see docs/RELEASING.md)
.goreleaser.yaml    how a tag becomes binaries, a release and a formula
cmd/journal/        entry point, build info, exit statuses
internal/config/    the shell-style config file, XDG paths, migration
internal/dateparse/ "last friday", "3 days ago", ISO dates
internal/journal/   the domain: entries, search, calendar, insights, daily recap
internal/gitrepo/   the git operations the tool needs
internal/standup/   the optional git-standup helper, parsed into a report
internal/editor/    locating and launching $EDITOR
internal/ui/        theme, styled output, Bubble Tea views
internal/cli/       flag parser, command table, dispatch
```

The domain package neither prints nor prompts; every style lives in one
`ui.Theme`. Two extension points, each a single edit: a **command** implements
`cli.Command` and joins `cli.Commands()` (help and dispatch follow from the
list); a **date expression** is one more rule in the `dateparse` table. The
interactive views are tested by injecting keystrokes into the Bubble Tea
models, so no terminal is needed.

Pull requests need exactly one of the `patch` / `minor` / `major` labels — it
drives the version of the release cut on merge. The pipeline is documented in
[docs/RELEASING.md](docs/RELEASING.md), and what a pull request is expected to
look like in [CONTRIBUTING.md](CONTRIBUTING.md).

## Acknowledgements

daily-journal began as a Go rewrite of
[journalot](https://github.com/jtaylortech/journalot) by
[jtaylortech](https://github.com/jtaylortech), originally a single bash script.
It has since been reshaped into subcommands and trimmed down, but the entry
format, the config syntax and the commit messages are unchanged — an existing
journalot journal works as is.

The interactive views are built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss); the calendar layout is
inspired by [tock](https://github.com/kriuchkov/tock)'s calendar view.

The git half of `journal daily` is
[git-standup](https://github.com/kamranahmedse/git-standup) by
[kamranahmedse](https://github.com/kamranahmedse), read as it prints. It stays an
optional dependency: everything else works without it.

## License

[MIT](LICENSE)
