#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
VERSION=${VERSION-0.1.0}
APP_NAME=PdmView
LICENSE_SOURCE="$ROOT/LICENSE"
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

[ "$(uname -s)" = Darwin ] || die "macOS is required for ZIP/PE verification"
command -v "$GO" >/dev/null 2>&1 || die "Go command not found: $GO"
command -v python3 >/dev/null 2>&1 || die "Python 3 is required for the Windows icon check"
[ -s "$LICENSE_SOURCE" ] || die "repository license missing or empty: $LICENSE_SOURCE"

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
python3 "$ROOT/scripts/check-windows-icon.py" "$STAGE/$APP_NAME.exe" >"$AUDIT/windows-icon.txt" ||
	die "PE product icon check failed"

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

NOTICES="$STAGE/ThirdPartyNotices"
mkdir "$NOTICES"
sh "$ROOT/scripts/package-notices.sh" windows "$NOTICES" "$STAGE/$APP_NAME.exe" "$AUDIT"
cp "$LICENSE_SOURCE" "$STAGE/LICENSE"

EXE_SHA=$(shasum -a 256 "$STAGE/$APP_NAME.exe" | awk '{ print $1 }')
(
	cd "$STAGE"
	/usr/bin/zip -X -q -r "$ZIP_NAME" "$APP_NAME.exe" LICENSE ThirdPartyNotices
)
unzip -t "$STAGE/$ZIP_NAME" >"$AUDIT/unzip-test.txt" ||
	die "zip integrity check failed"
unzip -Z1 "$STAGE/$ZIP_NAME" >"$AUDIT/zip-entries.txt"
zipinfo -l "$STAGE/$ZIP_NAME" |
	awk '$1 ~ /^[-dl][rwxStTs-]{9}$/ { print substr($1,1,1) "\t" $NF }' >"$AUDIT/zip-types.txt"
if awk -F '\t' '$1 != "-" && $1 != "d" { found = 1 } END { exit found ? 0 : 1 }' "$AUDIT/zip-types.txt"; then
	die "zip contains symlinks or special files"
fi
{
	printf '%s\n' "$APP_NAME.exe"
	printf '%s\n' "LICENSE"
	while IFS= read -r name; do
		printf 'ThirdPartyNotices/%s\n' "$name"
	done <"$AUDIT/notices-files.txt"
} >"$AUDIT/zip-expected.txt"
LC_ALL=C sort "$AUDIT/zip-expected.txt" >"$AUDIT/zip-expected.sorted.txt"
awk -F '\t' '$1 == "-" { print $2 }' "$AUDIT/zip-types.txt" |
	LC_ALL=C sort >"$AUDIT/zip-regular.txt"
cmp -s "$AUDIT/zip-expected.sorted.txt" "$AUDIT/zip-regular.txt" ||
	die "zip regular files do not match the exact whitelist"
printf 'ThirdPartyNotices/\n' >"$AUDIT/zip-expected-dirs.txt"
awk -F '\t' '$1 == "d" { print $2 }' "$AUDIT/zip-types.txt" |
	LC_ALL=C sort >"$AUDIT/zip-dirs.txt"
LC_ALL=C sort "$AUDIT/zip-expected-dirs.txt" >"$AUDIT/zip-expected-dirs.sorted.txt"
cmp -s "$AUDIT/zip-expected-dirs.sorted.txt" "$AUDIT/zip-dirs.txt" ||
	die "zip directory entries do not match the packaging manifest"

ZIP_EXE_SHA=$(unzip -p "$STAGE/$ZIP_NAME" "$APP_NAME.exe" | shasum -a 256 | awk '{ print $1 }')
[ "$EXE_SHA" = "$ZIP_EXE_SHA" ] || die "zip payload hash does not match the executable"
LICENSE_SHA=$(shasum -a 256 "$LICENSE_SOURCE" | awk '{ print $1 }')
ZIP_LICENSE_SHA=$(unzip -p "$STAGE/$ZIP_NAME" LICENSE | shasum -a 256 | awk '{ print $1 }')
[ "$LICENSE_SHA" = "$ZIP_LICENSE_SHA" ] ||
	die "zip LICENSE hash does not match the repository license"
while IFS= read -r name; do
	zip_notice_sha=$(unzip -p "$STAGE/$ZIP_NAME" "ThirdPartyNotices/$name" |
		shasum -a 256 | awk '{ print $1 }')
	source_notice_sha=$(shasum -a 256 "$NOTICES/$name" | awk '{ print $1 }')
	[ "$zip_notice_sha" = "$source_notice_sha" ] ||
		die "zip notice hash does not match: $name"
done <"$AUDIT/notices-files.txt"
ZIP_SHA=$(shasum -a 256 "$STAGE/$ZIP_NAME" | awk '{ print $1 }')
NOTICE_COUNT=$(wc -l <"$AUDIT/notices-files.txt" | tr -d ' ')
{
	printf 'file: %s\n' "$FILE_INFO"
	printf 'exe-sha256: %s\n' "$EXE_SHA"
	printf 'zip-sha256: %s\n' "$ZIP_SHA"
	cat "$AUDIT/windows-icon.txt"
	printf 'notices: %s files\n' "$NOTICE_COUNT"
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
