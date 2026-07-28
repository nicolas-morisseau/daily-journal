# Releasing

Releases are cut in two steps, deliberately separate: something decides the
version and tags it, and something else turns a tag into a release. A release
therefore never depends on going through a pull request — pushing a tag by hand
does the same thing.

Tags are the bare semantic version — `1.4.0`, never `v1.4.0`. That is what the
SemVer spec calls a version; the widespread `v` prefix is a git convention this
project does without, so the tag, the release and `journal --version` all read
the same. Everything that matches a tag assumes it: the trigger in
`release.yml`, its validation step, and the `[0-9]*` glob both `tag.yml` and
`next-version.sh` use to find the newest one.

## From a merged pull request

Every pull request into `main` carries exactly one label saying how the version
should move:

| Label | 0.1.3 becomes | For |
| --- | --- | --- |
| `patch` | 0.1.4 | a fix, no behaviour change |
| `minor` | 0.2.0 | a new command, option or behaviour |
| `major` | 1.0.0 | something that breaks an existing usage |

The label is a required check: `pull-request.yml` fails the pull request until
exactly one of the three is set, alongside `go vet`, `gofmt` and the tests.

On merge, `tag.yml` reads the same label, works out the next version from the
newest version tag, tags the merge commit, and calls `release.yml`.

## From a tag

```bash
git tag -a 1.4.0 -m "1.4.0" && git push origin 1.4.0
```

`release.yml` also listens for `*.*.*` tag pushes, so this releases without any
pull request. The tag must be exactly three numbers: `1.4.0` yes, `1.4` and
`1.4.0-rc1` no. Widening the pattern in `release.yml` is a one-line change if
release candidates are ever wanted — `.goreleaser.yaml` already sets
`prerelease: auto` to handle them.

## What a release does

`release.yml` hands over to [GoReleaser](https://goreleaser.com)
(`.goreleaser.yaml`), which:

1. runs the tests again in its `before` hooks,
2. builds `journal` for macOS and Linux, amd64 and arm64, with the version baked
   in through `-X main.version=`,
3. attaches the four tarballs and a `checksums.txt` to a new GitHub release,
   with generated notes,
4. regenerates `Formula/daily-journal.rb` — version, per-platform URLs and
   hashes — and commits it to
   [nicolas-morisseau/homebrew-tap](https://github.com/nicolas-morisseau/homebrew-tap).

That formula is generated output: never edit it by hand, the next release
overwrites it. It lives in a shared tap rather than in this repository, so
`brew tap nicolas-morisseau/tap` works without an explicit URL, nothing
generated is committed here, and the tap can carry other tools later.

A formula rather than a cask, deliberately: Homebrew quarantines what a cask
downloads, and macOS kills a quarantined binary carrying only the ad-hoc
signature the Go linker applies — a silent SIGKILL, exit 137. Formulae are not
quarantined, and they work on Linux, which casks do not.

## How the pieces fit

```
.github/scripts/bump-from-labels.sh   the label rule, used by both workflows below
.github/scripts/next-version.sh       latest version tag + bump -> next tag

pull-request.yml   on a pull request      the gate: one release label, vet, gofmt, tests
tag.yml            on a merged PR         label -> version -> tag, then calls release.yml
release.yml        on a *.*.* tag         build, publish, update the formula
                   or when called
```

The bump rule lives in one place, called by the gate and by the tagging, so a
pull request cannot pass the gate and then fail to release.

`tag.yml` calls `release.yml` rather than letting the tag push trigger it,
because a tag pushed with the default `GITHUB_TOKEN` does not start a workflow —
GitHub suppresses that to avoid loops. A tag pushed by hand does trigger it,
which is why both paths exist.

## Repository setup

The three labels have to exist, and `main` has to require the checks:

```bash
gh label create patch --description "Release: 0.1.3 -> 0.1.4" --color 0e8a16
gh label create minor --description "Release: 0.1.3 -> 0.2.0" --color 1d76db
gh label create major --description "Release: 0.1.3 -> 1.0.0" --color b60205
```

Then, in Settings → Rules → Rulesets, protect `main` with **Require a pull
request before merging** and require the `release label` and `test` status
checks.

Nothing has to bypass that ruleset: a release only writes to the tap repository,
never to `main` here.

## The tap

The formula is published to `nicolas-morisseau/homebrew-tap`, a repository whose
name is what `brew tap nicolas-morisseau/tap` expands to. It only needs to exist
with a `main` branch — GoReleaser creates `Formula/` on the first release. Being
public is what lets anyone install without authenticating.

`github.token` is scoped to this repository and cannot write to the tap, so the
release carries its own credential: an SSH deploy key on `homebrew-tap`, with
write access and no passphrase. Its private half is the `HOMEBREW_TAP_SSH_KEY`
secret here; `.goreleaser.yaml` points `repository.git` at it, and GoReleaser
pushes over SSH instead of through the API.

A deploy key rather than a personal access token because it does not expire,
grants nothing beyond that one repository, and does not depend on a user
account. GoReleaser writes it to a temporary file and invokes `ssh` itself, so
the workflow sets up no agent.

Replacing it, should it ever leak:

```bash
ssh-keygen -t ed25519 -N "" -C "goreleaser: daily-journal -> homebrew-tap" -f /tmp/tap_key
gh api repos/nicolas-morisseau/homebrew-tap/keys -X POST \
  -f title="goreleaser (daily-journal)" -f key="$(cat /tmp/tap_key.pub)" -F read_only=false
gh secret set HOMEBREW_TAP_SSH_KEY --repo nicolas-morisseau/daily-journal < /tmp/tap_key
rm /tmp/tap_key /tmp/tap_key.pub
```

Then delete the old key from the tap's Settings → Deploy keys. If the key is
missing or revoked, a release publishes the binaries and then fails on the
formula push, leaving the tap one version behind.

## Checking the pipeline without publishing

```bash
goreleaser check                        # validate .goreleaser.yaml
goreleaser release --snapshot --clean   # build everything into dist/, publish nothing
```

`goreleaser check` currently exits non-zero: `brews` is deprecated in favour of
`homebrew_casks`, which is exactly what this project cannot use. The message to
look for is "configuration is valid, but uses deprecated properties" — anything
else is a real problem. `goreleaser release` is unaffected and only warns, which
is why the release workflow does not run `check`. GoReleaser removes deprecated
options on major versions only, so this holds for the whole of v2;
`release.yml` pins `~> v2`. At v3 the choice is to notarise the binaries with a
paid Developer ID, or to generate the formula here instead.

```bash
# what a label would produce, against the tags you have
echo '["patch"]' | .github/scripts/bump-from-labels.sh
.github/scripts/next-version.sh minor
```

A snapshot writes the formula it would push to `dist/homebrew/Formula/`, which is
the quickest way to see the effect of a config change.
