# Publishing to the Connect IQ Store

Short version: **the upload cannot be automated.** Everything up to it can, and is.

## There is no API

Garmin publishes no store upload or publishing API. Checked in August 2026:

- The [developer program index](https://developer.garmin.com/) lists every Garmin API
  programme — Connect IQ SDK, Garmin Connect, Health, Golf, Marine, ANT, FIT, Fleet,
  inReach, Aviation. None of them publishes apps. (The Garmin Connect Developer Program
  is the fitness-data API, and is unrelated.)
- Garmin documents the process as a web form, with screenshots:
  [Publishing to the Store](https://developer.garmin.com/connect-iq/core-topics/publishing-to-the-store/)
  and [Submit an App](https://developer.garmin.com/connect-iq/submit-an-app/).
- The dashboard sits behind Garmin SSO and exposes no documented machine endpoint.
- Developers have asked for one:
  [How can I automate deployment to the App Store](https://forums.garmin.com/developer/connect-iq/f/discussion/211559/how-can-i-automate-deployment-to-the-app-store-not-just-build-or-release).
  Open since 2019, still "+1" replies in 2026, no Garmin answer.

No community tool fills the gap either. The Connect IQ tooling that exists
([matco/connectiq-tester](https://github.com/matco/connectiq-tester),
[action-connectiq-tester](https://github.com/matco/action-connectiq-tester),
[openhab-garmin's CI](https://github.com/openhab/openhab-garmin/blob/main/.github/workflows/ci-build.yaml))
builds and tests. None uploads. Garmin's own GitHub org publishes no CI action.

Scripting the dashboard is the only remaining route, and it is a bad one. Garmin's
[Terms of Use](https://www.garmin.com/en-US/legal/terms-of-use/) prohibit "using any
process, whether automated or manual, that accesses, copies, or scrapes content from the
Site through any means not purposely made available through the Site". That wording is
aimed at scraping rather than form submission, so it is arguably not a direct ban — but
there is no safe harbour either, and it would mean driving Garmin SSO with stored
credentials. Not worth it for a step that runs a few times a year.

## What is automated

`scripts/release-watch.sh` does everything the store form does not:

```bash
scripts/release-watch.sh 1.0.4          # bump, export, verify
scripts/release-watch.sh                # re-export the current version
scripts/release-watch.sh --beta 1.0.4   # beta build, under its own app id
```

It finds the SDK, sets `JAVA_HOME`, signs with `developer_key.der`, writes to `build/`
(never `dist/`, which goreleaser deletes), and prints the two values that decide whether
an upload is accepted: the version, which may only ever increase, and the app id, which
must match the published listing.

The relay half is fully automated — see [RELEASING.md](../RELEASING.md).

## Updating a published app

1. `scripts/release-watch.sh <version>` — the version must be higher than anything
   already uploaded. The store rejects a number it has seen.
2. Sign in at <https://apps-developer.garmin.com/en-US/developer/dashboard>.
3. Open the **existing** app, then **Upload New Version**. Do not start a new submission.
4. Enter the release notes. This is the only field that must be filled in every time;
   description, screenshots, icon and category persist.
5. Submit. Review takes roughly 72 hours. The current version stays live for users while
   the new one is in review.

The signing key must be the same one used for the original submission. A binary signed
with a different key is rejected, and `developer_key.der` is git-ignored — losing it is
unrecoverable.

## Beta apps, and the trap in them

A beta upload is a **separate store record**, private to your account, not reviewed, and
not publicly linkable. Garmin's
[Beta Apps](https://developer.garmin.com/connect-iq/core-topics/beta-apps/) page requires
it to carry **its own app id**:

> "To use this feature, you will need to create an alternate app id in your manifest using
> a UUID creator … When you are ready to release the app, change the app id to your
> production version in the manifest and upload it without checking the 'Beta App'
> checkbox."

The app id is the key binding a binary to one store record, and the store enforces it:
uploading a mismatched id fails with *"The app ID contained in the manifest file differs
from the ID originally registered for this application"*
([forum thread](https://forums.garmin.com/developer/connect-iq/f/connect-iq-web-store/437376/uuid-not-accepted)).

**Uploading the production app id as a beta is therefore dangerous.** Garmin documents
neither the collision behaviour nor a recovery path; the one thread of a developer stuck
after a beta/production id mix-up ends without a self-service fix. Either the upload is
rejected, or the id is claimed by the beta record and future updates to the public
listing break. This is unverified and not worth verifying experimentally.

`--beta` handles it: the beta id lives in `watch/.beta-app-id` (generated on first use,
untracked — a second 32-character hex id in a tracked file would trip
`scripts/check-no-leaks.sh`), it is swapped into the manifest only for that export, and a
trap on `EXIT INT TERM HUP` restores the manifest afterwards. The signal list matters:
bash does not run an `EXIT` trap when killed by an untrapped signal, so without it a
Ctrl-C during the two-minute export would leave the beta id in the working tree.
