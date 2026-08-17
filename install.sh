#!/usr/bin/env bash
#
# Claude Pulse relay installer.
#
#   curl -fsSL https://raw.githubusercontent.com/biokraft/claude-pulse/main/install.sh | bash
#
# Installs claude-pulse-relay into a bin directory on your PATH.
#
# It downloads a released binary for this platform and verifies it against the
# release's checksums.txt. Building from source is the fallback, used when the
# platform has no published binary, when a specific ref is requested, or when
# the script is run from a checkout — in which case it builds that checkout,
# which is what the user is looking at.
#
# Environment overrides:
#   PREFIX     install directory      (default: ~/.local/bin, or /usr/local/bin if already on PATH)
#   VERSION    release tag to install (default: the latest release)
#   FROM_SOURCE=1  skip the download and build from source
#   REF        git ref to build       (default: main; implies FROM_SOURCE)
#   REPO       git URL to clone       (default: https://github.com/biokraft/claude-pulse.git)

set -euo pipefail

REPO="${REPO:-https://github.com/biokraft/claude-pulse.git}"
SLUG="${SLUG:-biokraft/claude-pulse}"
# Overridable so the download path can be exercised against a local server;
# also lets someone install from a mirror.
API_BASE="${API_BASE:-https://api.github.com/repos/$SLUG}"
DOWNLOAD_BASE="${DOWNLOAD_BASE:-https://github.com/$SLUG/releases/download}"
# Whether REF was asked for matters, not just its value: the default is only a
# fallback for building, while an explicit one means "install exactly this".
REF_EXPLICIT="${REF:-}"
REF="${REF:-main}"
GO_MIN_MAJOR=1
GO_MIN_MINOR=25

# ---------------------------------------------------------------- output ----

# The Anthropic palette, same hex values the watch app uses (see
# watch/source/views/Chrome.mc) so the terminal and the wrist look related.
# 24-bit escapes where the terminal supports them, nearest xterm-256 otherwise:
# the clay accent has no good 16-colour equivalent, and approximating it with
# yellow is what makes a CLI look generic.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  case "${COLORTERM:-}" in
    truecolor|24bit)
      CLAY=$'\033[38;2;204;122;86m'      # #CC7A56 — accent
      CREAM=$'\033[38;2;247;245;242m'    # #F7F5F2 — primary text
      MUTED=$'\033[38;2;169;163;153m'    # #A9A399 — secondary text
      SAGE=$'\033[38;2;111;154;106m'     # #6F9A6A — success
      RUST=$'\033[38;2;193;90;62m'       # #C15A3E — error
      ;;
    *)
      CLAY=$'\033[38;5;173m'; CREAM=$'\033[38;5;255m'; MUTED=$'\033[38;5;247m'
      SAGE=$'\033[38;5;108m'; RUST=$'\033[38;5;167m'
      ;;
  esac
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
  CLAY=""; CREAM=""; MUTED=""; SAGE=""; RUST=""; BOLD=""; DIM=""; RESET=""
fi

step()  { printf '%s==>%s %s%s%s\n' "$CLAY$BOLD" "$RESET" "$CREAM$BOLD" "$1" "$RESET"; }
info()  { printf '%s    %s%s\n' "$MUTED" "$1" "$RESET"; }
warn()  { printf '%s  ! %s%s%s\n' "$CLAY$BOLD" "$RESET$MUTED" "$1" "$RESET" >&2; }
ok()    { printf '%s  ✓ %s%s%s\n' "$SAGE$BOLD" "$RESET$CREAM" "$1" "$RESET"; }
die()   { printf '%s  ✗ %s%s%s\n' "$RUST$BOLD" "$RESET$CREAM" "$1" "$RESET" >&2; exit 1; }

on_path() { case ":$PATH:" in *":$1:"*) return 0 ;; *) return 1 ;; esac; }

# --------------------------------------------------------- preflight ----

case "$(uname -s)" in
  Darwin|Linux) ;;
  *) die "unsupported platform: $(uname -s). The relay runs on macOS and Linux." ;;
esac

command -v git >/dev/null 2>&1 || die "git is required but not installed."

# Go is checked only on the source path, further down: requiring it here would
# turn a toolchain that a downloaded binary never needs into an install blocker.
require_go() {
  if ! command -v go >/dev/null 2>&1; then
    die "Go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ is required to build from source.
       macOS: brew install go
       Linux: https://go.dev/doc/install
       Or install a released binary instead: unset FROM_SOURCE and REF."
  fi
  go_version="$(go env GOVERSION 2>/dev/null || echo unknown)"
  gv="${go_version#go}"
  gv_major="${gv%%.*}"
  gv_rest="${gv#*.}"
  gv_minor="${gv_rest%%.*}"
  if [ "${gv_major:-0}" -lt "$GO_MIN_MAJOR" ] ||
     { [ "${gv_major:-0}" -eq "$GO_MIN_MAJOR" ] && [ "${gv_minor:-0}" -lt "$GO_MIN_MINOR" ]; }; then
    die "Go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ is required to build from source, found ${go_version}."
  fi
}

# Pick an install directory. An existing install wins over the defaults, so an
# upgrade replaces the binary in place rather than leaving a second copy for
# PATH order to choose between. Otherwise prefer a directory already on PATH so
# the binary is runnable the moment this script finishes.
existing=""
if [ -n "${PREFIX:-}" ]; then
  bindir="$PREFIX"
  [ -x "$bindir/claude-pulse-relay" ] && existing="$bindir/claude-pulse-relay"
elif found="$(command -v claude-pulse-relay 2>/dev/null)" && [ -n "$found" ]; then
  existing="$found"
  bindir="$(cd "$(dirname "$found")" && pwd)"
elif on_path "$HOME/.local/bin"; then
  bindir="$HOME/.local/bin"
elif [ -w /usr/local/bin ] && on_path /usr/local/bin; then
  bindir="/usr/local/bin"
else
  bindir="$HOME/.local/bin"
fi
target="$bindir/claude-pulse-relay"
[ -z "$existing" ] && [ -x "$target" ] && existing="$target"

# --------------------------------------------------- existing install ----

pulse_home="${CLAUDE_PULSE_HOME:-$HOME/.claude-pulse}"
receipt="$pulse_home/installed-version"

# The previous version comes from a receipt file, never by running the old
# binary. Builds before commit 8dce8b8 have no `version` subcommand: the
# argument falls through to flag.Parse, which ignores non-flag arguments, and
# the relay starts — asking one for its version would hang this script.
old_version=""
if [ -n "$existing" ]; then
  if [ -r "$receipt" ]; then
    old_version="$(head -n 1 "$receipt" | tr -d '[:space:]')"
  fi
  [ -n "$old_version" ] || old_version="unknown (pre-upgrade build)"
fi

# A service keeps running the old binary until it is restarted, so an upgrade
# has to know whether one exists.
if [ "$(uname -s)" = "Darwin" ]; then
  service_unit="$HOME/Library/LaunchAgents/com.claudepulse.relay.plist"
else
  service_unit="$HOME/.config/systemd/user/claude-pulse-relay.service"
fi
has_service=0
if [ -f "$service_unit" ]; then
  # Only claim the service if it actually runs the binary being replaced.
  # Installing to a different PREFIX otherwise silently repoints the running
  # service at the new location — observed while testing this script with
  # PREFIX=/tmp, which left the real service executing out of /tmp.
  if grep -q "$target" "$service_unit" 2>/dev/null; then
    has_service=1
  else
    warn "a relay service exists but runs a different binary; leaving it alone"
    info "$(grep -o '/[^"<[:space:]]*claude-pulse-relay' "$service_unit" 2>/dev/null | head -1)"
  fi
fi

stop_service() {
  if [ "$(uname -s)" = "Darwin" ]; then
    launchctl unload "$service_unit" >/dev/null 2>&1 || true
  else
    systemctl --user stop claude-pulse-relay >/dev/null 2>&1 || true
  fi
}

# ------------------------------------------------------------ sources ----

script_dir=""
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

cleanup_dir=""
# Must not end on a failed test: the trap's status becomes the script's.
cleanup() { if [ -n "$cleanup_dir" ]; then rm -rf "$cleanup_dir"; fi; }
trap cleanup EXIT

# Building is the fallback, not the default, but three situations still call
# for it: an explicit request, a specific ref, or running from a checkout —
# where the source in front of the user is the thing they meant to install.
build_from_source=0
[ "${FROM_SOURCE:-0}" = "1" ] && build_from_source=1
[ -n "${REF_EXPLICIT:-}" ] && build_from_source=1
if [ -n "$script_dir" ] && [ -f "$script_dir/relay/go.mod" ]; then
  build_from_source=1
fi

# The platform triple used by the release archives. An unrecognised pair is not
# an error: it just means there is no binary to download and source is the only
# route.
asset_os=""
asset_arch=""
case "$(uname -s)" in
  Darwin) asset_os="darwin" ;;
  Linux)  asset_os="linux" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) asset_arch="amd64" ;;
  arm64|aarch64) asset_arch="arm64" ;;
esac

if [ "$build_from_source" = "0" ] && { [ -z "$asset_os" ] || [ -z "$asset_arch" ]; }; then
  warn "no released binary for $(uname -s)/$(uname -m) — building from source instead"
  build_from_source=1
fi

if [ -n "$existing" ]; then
  step "Upgrading claude-pulse-relay"
  info "found $existing ($old_version)"
  if [ "$has_service" = "1" ]; then
    info "service installed — it will be restarted on the new version"
  fi
fi

mkdir -p "$bindir"

# Stage beside the target, never over it: the same filesystem keeps the final
# move atomic, and a failed install cannot truncate a binary that works.
staged="$target.new"
backup="$target.bak"
rm -f "$staged"

# ----------------------------------------------------------- download ----

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    # Never silently skip verification: an unverified binary is exactly what
    # the checksum exists to prevent.
    return 1
  fi
}

download_release() {
  command -v curl >/dev/null 2>&1 || { warn "curl is not installed"; return 1; }
  command -v tar  >/dev/null 2>&1 || { warn "tar is not installed"; return 1; }

  local tag="${VERSION:-}"
  if [ -z "$tag" ]; then
    tag="$(curl -fsSL "$API_BASE/releases/latest" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
  fi
  [ -n "$tag" ] || { warn "could not determine the latest release"; return 1; }

  local version="${tag#v}"
  local archive="claude-pulse-relay_${version}_${asset_os}_${asset_arch}.tar.gz"
  local base="$DOWNLOAD_BASE/$tag"

  local tmp
  tmp="$(mktemp -d)"
  cleanup_dir="$tmp"

  step "Downloading claude-pulse-relay $tag"
  info "$archive"
  curl -fsSL -o "$tmp/$archive" "$base/$archive" 2>/dev/null \
    || { warn "no binary published for ${asset_os}/${asset_arch} in $tag"; return 1; }
  curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" 2>/dev/null \
    || { warn "$tag has no checksums.txt"; return 1; }

  local want got
  want="$(awk -v f="$archive" '$2 == f || $2 == "*" f {print $1}' "$tmp/checksums.txt" | head -1)"
  [ -n "$want" ] || { warn "$archive is not listed in checksums.txt"; return 1; }
  got="$(sha256_of "$tmp/$archive")" \
    || die "neither sha256sum nor shasum is available, so the download cannot be verified.
       Install one, or re-run with FROM_SOURCE=1 to build instead."
  if [ "$want" != "$got" ]; then
    die "checksum mismatch for $archive.
       expected $want
       actual   $got
       Refusing to install. Report this at https://github.com/$SLUG/issues"
  fi
  ok "checksum verified"

  tar -xzf "$tmp/$archive" -C "$tmp" || { warn "could not extract $archive"; return 1; }
  [ -f "$tmp/claude-pulse-relay" ] && [ ! -L "$tmp/claude-pulse-relay" ] \
    || { warn "the archive did not contain the expected binary"; return 1; }

  mv "$tmp/claude-pulse-relay" "$staged" || return 1
  chmod 755 "$staged"
  return 0
}

# ------------------------------------------------------------- build ----

build_from_git() {
  require_go
  command -v git >/dev/null 2>&1 || die "git is required to build from source."

  local src
  if [ -n "$script_dir" ] && [ -f "$script_dir/relay/go.mod" ]; then
    src="$script_dir"
    step "Building from this checkout"
    info "$src"
  else
    src="$(mktemp -d)"
    cleanup_dir="$src"
    step "Fetching claude-pulse ($REF)"
    git clone --depth 1 --branch "$REF" --quiet "$REPO" "$src" \
      || die "failed to clone $REPO at $REF"
  fi

  local build_version
  build_version="$(cd "$src" && git describe --tags --always 2>/dev/null || echo "$REF")"
  ( cd "$src/relay" && go build -trimpath \
      -ldflags "-X main.version=$build_version" \
      -o "$staged" ./cmd/claude-pulse-relay ) \
    || die "build failed"
}

if [ "$build_from_source" = "1" ]; then
  build_from_git
elif ! download_release; then
  warn "falling back to building from source"
  rm -f "$staged"
  build_from_git
fi

# Prove it executes before it is allowed to replace anything.
new_version="$("$staged" version 2>/dev/null | awk '{print $2}')"
if [ -z "$new_version" ]; then
  rm -f "$staged"
  die "the freshly built binary did not run — leaving the current install alone"
fi

# ------------------------------------------------------------- swap ----

restore_backup() {
  if [ -f "$backup" ]; then
    mv -f "$backup" "$target"
    warn "restored the previous binary at $target"
    if [ "$has_service" = "1" ]; then
      "$target" service install >/dev/null 2>&1 \
        && warn "restarted the service on the previous version" \
        || warn "could not restart the service — run: $target service install"
    fi
  fi
}

if [ "$has_service" = "1" ]; then
  stop_service
fi

if [ -n "$existing" ] && [ -f "$target" ]; then
  cp -p "$target" "$backup" || die "could not back up $target"
fi

mv -f "$staged" "$target" || { rm -f "$staged"; restore_backup; die "could not install to $target"; }

# Nothing is reported as installed until the service is back up, so a rollback
# never leaves a success message (or a version receipt) behind for a version
# that is no longer on disk.
restarted=0
if [ "$has_service" = "1" ]; then
  if "$target" service install >/dev/null 2>&1; then
    restarted=1
  else
    warn "the new version failed to start as a service — rolling back"
    restore_backup
    die "upgrade rolled back. Try running '$target' in a terminal to see the error."
  fi
fi

if [ -n "$existing" ]; then
  if [ "$old_version" = "$new_version" ]; then
    ok "reinstalled $new_version at $target"
  else
    ok "upgraded $old_version -> $new_version at $target"
  fi
else
  ok "installed $new_version at $target"
fi
[ "$restarted" = "1" ] && ok "restarted the service on $new_version"

# Record what is installed so the next upgrade can report the version without
# executing the old binary.
mkdir -p "$pulse_home" && chmod 700 "$pulse_home" 2>/dev/null || true
printf '%s\n' "$new_version" > "$receipt" 2>/dev/null || true

rm -f "$backup"

# ------------------------------------------------------ post-install ----

echo
if ! command -v claude-pulse-relay >/dev/null 2>&1; then
  warn "$bindir is not on your PATH. Add it:"
  info ""
  info "    echo 'export PATH=\"$bindir:\$PATH\"' >> ~/.zshrc && exec zsh"
  info ""
fi

if ! command -v cloudflared >/dev/null 2>&1; then
  warn "cloudflared not found — needed for the built-in tunnel."
  if [ "$(uname -s)" = "Darwin" ]; then
    info "    brew install cloudflared"
  else
    info "    https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
  fi
  info "    (or run the relay with --no-tunnel and supply your own HTTPS route)"
  echo
fi

if [ -n "$existing" ] && [ -f "$pulse_home/config.json" ]; then
  info "${DIM}config kept as-is: $pulse_home/config.json (same token, same history)${RESET}"
  echo
fi

if [ -n "$existing" ]; then
  # An upgrade needs no setup instructions — only what changed.
  if [ "$restarted" = "1" ]; then
    cat <<EOF
${CLAY}${BOLD}Done — the service is now running $new_version.${RESET}

  Restarting rotates the Cloudflare quick-tunnel URL, so the watch needs the new
  one. Scan the fresh QR code from the log:

$(if [ "$(uname -s)" = "Darwin" ]; then
    printf '       %s\n' "${BOLD}tail -f $pulse_home/relay.log${RESET}"
  else
    printf '       %s\n' "${BOLD}journalctl --user -u claude-pulse-relay -f${RESET}"
  fi)
EOF
  else
    cat <<EOF
${CLAY}${BOLD}Done — $new_version is installed.${RESET}

  Restart any relay you have running to pick it up:

       ${BOLD}claude-pulse-relay${RESET}

  ${DIM}Restarting rotates the tunnel URL, so re-scan the QR code afterwards.${RESET}
EOF
  fi
else
  cat <<EOF
${CLAY}${BOLD}Next steps${RESET}

  ${CLAY}1.${RESET} Start the relay — prints your pairing URL, token and a QR code:

       ${BOLD}claude-pulse-relay${RESET}

  ${CLAY}2.${RESET} Scan that QR code with your phone, then paste the URL and token into
     ${BOLD}Garmin Connect → Connect IQ apps → Claude Pulse → Settings${RESET}.

  ${CLAY}3.${RESET} Optional — keep it running across reboots, and feed the cost page:

       ${BOLD}claude-pulse-relay service install${RESET}   ${DIM}# launchd / systemd user service${RESET}
       ${BOLD}claude-pulse-relay hook install${RESET}      ${DIM}# Claude Code statusline hook${RESET}

  ${DIM}claude-pulse-relay help${RESET} lists every command.
  ${DIM}Re-run this installer any time to upgrade — your config is preserved.${RESET}
EOF
fi
