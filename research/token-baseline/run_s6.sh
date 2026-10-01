#!/usr/bin/env bash
# FEATURE-577 stage-S6 sampler.
#
# Runs the fixed task N times inside a dedicated workspace. The workspace has
# its own config.json / db/ / log/ (co-shell resolves these under the workspace
# root), so:
#   - the bbolt database never collides with a running co-shell instance;
#   - the llm-interaction log of the run stays isolated (no other traffic);
#   - the user's runtime data and the repository are left untouched.
#
# Usage:
#   bash research/token-baseline/run_s6.sh <label-prefix> <workspace-abs-dir> [runs]
# Example:
#   bash research/token-baseline/run_s6.sh s6s /tmp/fe577-s6/synth 3
#   bash research/token-baseline/run_s6.sh s6r /tmp/fe577-s6/repo  3

set -u

ROOT=/Users/direct3d/github/co-shell
BIN="$ROOT/work/co-shell-0.87.0.darwin.arm64"
OUT="$ROOT/research/token-baseline"
PREFIX="$1"
WD="$2"
N="${3:-3}"

for i in $(seq 1 "$N"); do
  FROM="$(date '+%Y-%m-%d %H:%M:%S')"
  ( cd "$WD" && "$BIN" run --workdir "$WD" \
      --prompt "$(cat "$OUT/fixed-task.txt")" ) > "$OUT/${PREFIX}-${i}.stdout" 2>&1
  RC=$?
  TO="$(date '+%Y-%m-%d %H:%M:%S')"
  printf 'workspace=%s\nexit=%s\nfrom=%s\nto=%s\ncommit=%s\nconfig_md5=%s\n' \
    "$WD" "$RC" "$FROM" "$TO" "$(cd "$ROOT" && git rev-parse HEAD)" \
    "$(md5 -q "$ROOT/config.json")" > "$OUT/${PREFIX}-${i}.meta"

  LOG="$WD/log/llm-interaction-$(date +%F).log"
  if [ -f "$LOG" ]; then
    python3 "$OUT/parse_llm_log.py" --log "$LOG" --from "$FROM" --to "$TO" \
      --out "$OUT/${PREFIX}-${i}"
  else
    echo "log not found: $LOG (exit=$RC)" >&2
  fi
done
