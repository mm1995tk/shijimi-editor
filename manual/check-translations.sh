#!/usr/bin/env bash
# Checks that the Japanese and English manual pages are in sync.
#
#   manual/check-translations.sh [BASE_REF]
#
# 1. Every page has both docs/<name>.ja.md and docs/<name>.en.md.
# 2. Each pair has the same number of headings, code fences and table rows.
# 3. With BASE_REF (e.g. origin/main), a page whose one language changed since
#    the merge base must have the other language changed too.
#
# To skip check 3 for a change that needs no translation (a typo fix, say),
# put "[skip-i18n-check]" in the pull request title (see manual.yml).
set -euo pipefail

cd "$(dirname "$0")/docs"
base="${1:-}"
fail=0
err() { echo "error: $*" >&2; fail=1; }

count() { grep -cE "$2" "$1" || true; }

names=$(ls ./*.ja.md ./*.en.md | sed -E 's|^\./||; s|\.(ja\|en)\.md$||' | sort -u)

for n in $names; do
  ja="$n.ja.md"
  en="$n.en.md"
  if [ ! -f "$ja" ]; then err "$ja is missing (only $en exists)"; continue; fi
  if [ ! -f "$en" ]; then err "$en is missing (only $ja exists)"; continue; fi

  for spec in 'headings:^#{1,6} ' 'code fences:^ *(```|~~~)' 'table rows:^ *\|'; do
    label=${spec%%:*}
    re=${spec#*:}
    a=$(count "$ja" "$re")
    b=$(count "$en" "$re")
    [ "$a" = "$b" ] || err "$n: number of $label differs (ja=$a, en=$b)"
  done

  if [ -n "$base" ]; then
    mb=$(git merge-base "$base" HEAD)
    ja_changed=$(git diff --name-only "$mb" -- "$ja" | wc -l)
    en_changed=$(git diff --name-only "$mb" -- "$en" | wc -l)
    if [ "$ja_changed" != "$en_changed" ]; then
      if [ "$ja_changed" -gt 0 ]; then
        err "$ja changed but $en did not. Update the English page too"
      else
        err "$en changed but $ja did not. Update the Japanese page too"
      fi
    fi
  fi
done

exit $fail
