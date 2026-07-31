# Contributing

Thanks for taking an interest. Bug reports, fixes and device-compatibility reports are all
welcome.

## Before you start

For anything larger than a bug fix, open an issue first and describe what you want to
change. It saves you building something that turns out not to fit — this is a small,
deliberately narrow app, and "no" is a normal answer to a feature request.

## Setting up

The relay needs Go 1.25+. The watch app needs the
[Connect IQ SDK](https://developer.garmin.com/connect-iq/sdk/), a JDK, and a one-time
developer key:

```bash
./scripts/gen-dev-key.sh    # writes developer_key.der, which is git-ignored
```

Both test suites are in the README's Development section, and both run in CI on every push.
Run them locally before opening a pull request.

## Working on the relay

- Keep packages under `relay/internal/` focused; each has its own tests next to it.
- `go test -race ./...` must pass. New behaviour needs a test.
- No new dependencies without a good reason. The relay is deliberately close to the
  standard library.

## Working on the watch app

- Layout changes must follow [`docs/design-spec.md`](docs/design-spec.md), which pins every
  measurement to the mockup's 400 px reference screen. Scale from
  `dc.getWidth() / 400.0` — never hard-code device pixels.
- Colours come from `watch/source/views/Chrome.mc`. Don't re-derive palette values per view.
- Pure layout and formatting logic belongs in a static function with a unit test
  (see `watch/tests/`). Drawing code itself is not unit-testable, so keep it thin.
- Screenshots: `scripts/shoot-pages.sh` captures all three pages from the simulator.
  [`CLAUDE.md`](CLAUDE.md) documents its workarounds and the fake-data seed used for them.

## Pull requests

- One logical change per pull request.
- Write commit messages in the imperative mood, with a body explaining *why* when the
  reason isn't obvious from the diff.
- Say which device or simulator target you tested on. Device coverage is 70 models and
  nobody owns them all — being explicit about what you verified is more useful than
  implying it all works.

## Code of conduct

Participation is covered by the [Code of Conduct](CODE_OF_CONDUCT.md).
