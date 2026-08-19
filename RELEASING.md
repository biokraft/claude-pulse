# Releasing

Two artifacts ship from this repository and they release differently. The relay is
automated end to end. The watch app is not, and cannot be: Connect IQ packages are built
by Garmin's closed SDK with a signing key that must never be in CI, and the upload is a
manual step in Garmin's dashboard. Nothing here can enforce that half, so it is a
checklist instead.

## The relay

Nothing is versioned by hand.

1. Land commits on `main` using [Conventional Commits](https://www.conventionalcommits.org)
   — `feat:`, `fix:`, `docs:`, `perf:`, `refactor:`. Anything else is left out of the
   changelog.
2. `release-please` keeps a release pull request open with the next version and the
   changelog entries for everything merged since the last release. Review it like any
   other PR.
3. Merging it tags `vX.Y.Z` and creates the GitHub Release.
4. That tag triggers `release.yml`, which builds four binaries with goreleaser, attaches
   them with a `checksums.txt`, attests their provenance, and pushes the Homebrew cask.

`install.sh` picks up the new release automatically — it resolves the latest tag from
the GitHub API.

### Cutting one by hand

Only when release-please cannot, such as the first release after adopting it:

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3        # this is what triggers the binaries
```

Then bring `.release-please-manifest.json` in line, or the next automated release will
propose a version that has already shipped.

## The watch app

Required whenever anything under `watch/` changes. The store rejects a version number it
has already seen, so this number only ever goes up.

```bash
scripts/release-watch.sh 1.0.4          # bump, export to build/, verify
scripts/release-watch.sh --beta 1.0.4   # a beta build, under its own app id
```

Then upload it: dashboard → the
[**existing** app](https://apps.garmin.com/apps/366c5488-cd20-4299-b498-8c5cff30bcba) →
**Upload New Version**. Never "Add
Beta App" for a release, and never a beta upload carrying the production app id —
[docs/garmin-release.md](docs/garmin-release.md) explains why that one is hard to undo.

There is no way to automate the upload itself: Garmin publishes no store API, no
community tool exists, and scripting the dashboard runs into their Terms of Use. The
research behind that conclusion, with sources, is in
[docs/garmin-release.md](docs/garmin-release.md).

The relay and the watch app carry **different version numbers on purpose**. They are
separate artifacts on separate release cadences, and the watch app's numbering is
constrained by what has already been published to the store. A release that changes only
the relay leaves the watch app where it is, and says so in its notes.

## Secrets

| Secret | Used by | Without it |
| --- | --- | --- |
| `RELEASE_TOKEN` | `release-please.yml` | The job is skipped entirely and releases must be tagged by hand. It must be a PAT: a tag pushed with the automatic `GITHUB_TOKEN` does **not** start another workflow, so the release would be created with no binaries and nothing would explain why. |
| `TAP_TOKEN` | `release.yml` | The Homebrew cask is built but not pushed. Everything else still releases. `GITHUB_TOKEN` cannot stand in — it has no write access to the tap repository. |

Both are guarded, so a fork with neither still gets a working CI run and a working
release.

## Failure modes

- **The tag shape is written down twice** — in `release-please-config.json` and in the
  `tags: ["v*"]` trigger of `release.yml`. Change one without the other and releases keep
  appearing with no binaries attached.
- **Never delete or move a published tag.** The Homebrew cask and any `install.sh` run
  pin to it. To fix a bad release, land a `fix:` commit and cut the next patch.
- **Never hand-edit `CHANGELOG.md` above the beta section**, or the next release-please
  run will fight it.
- **Check the release actually has four `.tar.gz` files and a `checksums.txt`.** An
  install falls back to building from source when a platform's binary is missing, which
  looks like a slow install rather than an error.

## Verifying a release

```bash
gh release view v1.2.3
gh attestation verify --owner biokraft claude-pulse-relay_1.2.3_darwin_arm64.tar.gz
curl -fsSL https://raw.githubusercontent.com/biokraft/claude-pulse/main/install.sh | bash
claude-pulse-relay version
```
