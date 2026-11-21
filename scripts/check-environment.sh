#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
cd "$project_root"

printf 'Workspace: %s\n' "$project_root"
uname -sm
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git status --short --branch --untracked-files=no
else
    printf 'Git repository: unavailable\n'
fi

if command -v go >/dev/null 2>&1; then
    GOTOOLCHAIN=local go version
    GOTOOLCHAIN=local go env -json GOOS GOARCH CGO_ENABLED CC
else
    printf 'Go: unavailable\n'
fi

if [ "$(uname -s)" = Darwin ]; then
    sw_vers
    xcode-select -p || true
    xcrun --sdk macosx --show-sdk-version || true
    xcrun clang --version || true
fi

for tool in mysql docker podman; do
    if command -v "$tool" >/dev/null 2>&1; then
        "$tool" --version
    else
        printf '%s: unavailable\n' "$tool"
    fi
done

printf 'PDM sample metadata (workspace only):\n'
if [ "$(uname -s)" = Darwin ]; then
    samples=$(find -P . -type d -name .git -prune -o -type f -iname '*.pdm' -exec stat -f '%z bytes  %N' {} +)
else
    samples=$(find -P . -type d -name .git -prune -o -type f -iname '*.pdm' -exec stat -c '%s bytes  %n' {} +)
fi
printf '%s\n' "${samples:-No .pdm files found.}"
