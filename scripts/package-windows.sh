#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
VERSION=${VERSION-0.1.0}
APP_NAME=PdmView
OUT="$ROOT/.out/release"

die() {
	printf 'package-windows: %s\n' "$*" >&2
	exit 1
}

case "$VERSION" in
'' | *[!0-9.]*)
	die "VERSION must be a numeric version in x.y.z form (digits and dots only, no newlines)"
	;;
esac
printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ||
	die "VERSION must be a numeric version in x.y.z form (got: $VERSION)"

[ "$(uname -s)" = Darwin ] || die "macOS is required for ditto packaging"
command -v "$GO" >/dev/null 2>&1 || die "Go command not found: $GO"

VERSION_DIR="$OUT/$VERSION"
BASE_OUTPUT="$VERSION_DIR/windows-amd64"
mkdir -p "$VERSION_DIR"

ZIP_NAME="$APP_NAME-$VERSION-windows-amd64.zip"

STAGE=
on_exit() {
	status=$?
	trap - EXIT
	if [ "$status" -ne 0 ] && [ -n "${STAGE:-}" ] && [ -d "$STAGE" ]; then
		printf 'package-windows: staging retained at %s\n' "$STAGE" >&2
	fi
	exit "$status"
}
trap on_exit EXIT

STAGE=$(mktemp -d "$VERSION_DIR/.package-windows.XXXXXX") ||
	die "could not create a staging directory under $VERSION_DIR"
AUDIT="$STAGE/audit"
mkdir "$AUDIT"

printf 'Building %s %s windows/amd64\n' "$APP_NAME" "$VERSION"
(
	cd "$ROOT"
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		"$GO" build -mod=readonly -trimpath -buildvcs=false \
		-tags=mygo_noinspector,mygo_nowebview \
		-ldflags='-s -w -H=windowsgui' \
		-o "$STAGE/$APP_NAME.exe" ./cmd/pdmview
)
[ -f "$STAGE/$APP_NAME.exe" ] || die "build did not produce $APP_NAME.exe"

FILE_INFO=$(file -b "$STAGE/$APP_NAME.exe")
printf '%s\n' "$FILE_INFO" >"$AUDIT/file.txt"
printf '%s\n' "$FILE_INFO" | grep -q 'PE32+ executable (GUI) x86-64' ||
	die "unexpected PE format: $FILE_INFO"

"$GO" version -m "$STAGE/$APP_NAME.exe" >"$AUDIT/go-version-m.txt"
for setting in \
	'GOOS=windows' \
	'GOARCH=amd64' \
	'CGO_ENABLED=0' \
	'-trimpath=true' \
	'-tags=mygo_noinspector,mygo_nowebview'
do
	grep -Fq -- "$setting" "$AUDIT/go-version-m.txt" ||
		die "build metadata lacks $setting"
done
if grep -q 'vcs=' "$AUDIT/go-version-m.txt"; then
	die "build metadata contains VCS information"
fi
NM_OUT=$("$GO" tool nm "$STAGE/$APP_NAME.exe" 2>&1 || true)
printf '%s\n' "$NM_OUT" >"$AUDIT/nm.txt"
case "$NM_OUT" in
*'no symbols'*) ;;
*) die "binary symbol table was not stripped" ;;
esac
objdump -h "$STAGE/$APP_NAME.exe" >"$AUDIT/objdump-sections.txt" ||
	die "could not inspect PE sections"
if grep -q '\.debug_' "$AUDIT/objdump-sections.txt"; then
	die "binary still contains DWARF debug sections"
fi

for marker in \
	'WebView2Loader.dll' \
	'msedgewebview2.exe' \
	'CreateCoreWebView2EnvironmentWithOptions' \
	'CreateCoreWebView2Controller' \
	'WEBVIEW2_USER_DATA_FOLDER' \
	'WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS' \
	'WEBVIEW2_BROWSER_EXECUTABLE_FOLDER' \
	'WebView2 Runtime' \
	'WebView2 environment'
do
	if LC_ALL=C grep -a -F -q -- "$marker" "$STAGE/$APP_NAME.exe"; then
		die "binary contains WebView2 marker: $marker"
	fi
done
printf 'WebView2 markers: none\n' >"$AUDIT/webview2.txt"

/usr/bin/ditto -c -k --norsrc "$STAGE/$APP_NAME.exe" "$STAGE/$ZIP_NAME"
unzip -t "$STAGE/$ZIP_NAME" >"$AUDIT/unzip-test.txt" ||
	die "zip integrity check failed"
unzip -Z1 "$STAGE/$ZIP_NAME" >"$AUDIT/zip-entries.txt"
[ "$(wc -l <"$AUDIT/zip-entries.txt" | tr -d ' ')" -eq 1 ] ||
	die "zip must contain exactly one entry"
[ "$(sed -n '1p' "$AUDIT/zip-entries.txt")" = "$APP_NAME.exe" ] ||
	die "zip entry must be $APP_NAME.exe"
zipinfo -l "$STAGE/$ZIP_NAME" |
	awk '$1 ~ /^[-dl][rwxStTs-]{9}$/ { print substr($1,1,1) "\t" $NF }' >"$AUDIT/zip-types.txt"
[ "$(wc -l <"$AUDIT/zip-types.txt" | tr -d ' ')" -eq 1 ] ||
	die "zip metadata must contain exactly one entry"
awk -F '\t' 'NR != 1 || $1 != "-" || $2 != "PdmView.exe" { exit 1 }' "$AUDIT/zip-types.txt" ||
	die "zip entry is not a regular PdmView.exe"

EXE_SHA=$(shasum -a 256 "$STAGE/$APP_NAME.exe" | awk '{ print $1 }')
ZIP_EXE_SHA=$(unzip -p "$STAGE/$ZIP_NAME" "$APP_NAME.exe" | shasum -a 256 | awk '{ print $1 }')
[ "$EXE_SHA" = "$ZIP_EXE_SHA" ] || die "zip payload hash does not match the executable"
ZIP_SHA=$(shasum -a 256 "$STAGE/$ZIP_NAME" | awk '{ print $1 }')
{
	printf 'file: %s\n' "$FILE_INFO"
	printf 'exe-sha256: %s\n' "$EXE_SHA"
	printf 'zip-sha256: %s\n' "$ZIP_SHA"
	printf 'webview2-markers: none\n'
	printf 'vcs-info: none\n'
	printf 'symbols: stripped\n'
	printf 'dwarf: absent\n'
} >"$AUDIT/summary.txt"

if mkdir "$BASE_OUTPUT" 2>/dev/null; then
	OUTPUT="$BASE_OUTPUT"
else
	OUTPUT=$(mktemp -d "$VERSION_DIR/windows-amd64.XXXXXX") ||
		die "could not create a unique output directory under $VERSION_DIR"
fi
EXE="$OUTPUT/$APP_NAME.exe"
ZIP="$OUTPUT/$ZIP_NAME"
[ ! -e "$EXE" ] || die "refusing to overwrite existing $EXE"
[ ! -e "$ZIP" ] || die "refusing to overwrite existing $ZIP"
mv "$STAGE/$APP_NAME.exe" "$EXE"
[ -f "$EXE" ] && [ ! -e "$STAGE/$APP_NAME.exe" ] ||
	die "could not install executable without overwriting an existing file"
mv "$STAGE/$ZIP_NAME" "$ZIP"
[ -f "$ZIP" ] && [ ! -e "$STAGE/$ZIP_NAME" ] ||
	die "could not install zip without overwriting an existing file"
mv "$AUDIT" "$OUTPUT/audit"
rmdir "$STAGE" 2>/dev/null || true

printf 'Exe: %s\nZip: %s\n' "$EXE" "$ZIP"
printf 'Audit: %s\n' "$OUTPUT/audit"
printf 'SHA-256 (exe): %s\nSHA-256 (zip): %s\n' "$EXE_SHA" "$ZIP_SHA"
