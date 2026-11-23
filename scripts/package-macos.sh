#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
VERSION=${VERSION-0.1.0}
APP_NAME=PdmView
ICON_NAME="$APP_NAME.icns"
ICON_SOURCE="$ROOT/assets/$ICON_NAME"
LICENSE_SOURCE="$ROOT/LICENSE"
OUT="$ROOT/.out/release"

die() {
	printf 'package-macos: %s\n' "$*" >&2
	exit 1
}

case "$VERSION" in
'' | *[!0-9.]*)
	die "VERSION must be a numeric Apple version in x.y.z form (digits and dots only, no newlines)"
	;;
esac
printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ||
	die "VERSION must be a numeric Apple version in x.y.z form (got: $VERSION)"

[ "$(uname -s)" = Darwin ] || die "macOS is required for app-bundle signing and ditto"
command -v "$GO" >/dev/null 2>&1 || die "Go command not found: $GO"
[ -s "$ICON_SOURCE" ] || die "app icon missing or empty: $ICON_SOURCE"
[ -s "$LICENSE_SOURCE" ] || die "repository license missing or empty: $LICENSE_SOURCE"

VERSION_DIR="$OUT/$VERSION"
BASE_OUTPUT="$VERSION_DIR/macos-arm64"
mkdir -p "$VERSION_DIR"
if mkdir "$BASE_OUTPUT" 2>/dev/null; then
	OUTPUT="$BASE_OUTPUT"
else
	OUTPUT=$(mktemp -d "$VERSION_DIR/macos-arm64.XXXXXX") ||
		die "could not create a unique output directory under $VERSION_DIR"
fi

APP="$OUTPUT/$APP_NAME.app"
ZIP_NAME="$APP_NAME-$VERSION-macos-arm64.zip"
ZIP="$OUTPUT/$ZIP_NAME"

STAGE=
on_exit() {
	status=$?
	trap - EXIT
	if [ "$status" -ne 0 ] && [ -n "${STAGE:-}" ] && [ -d "$STAGE" ]; then
		printf 'package-macos: staging retained at %s\n' "$STAGE" >&2
	fi
	exit "$status"
}
trap on_exit EXIT

STAGE=$(mktemp -d "$OUTPUT/.package-macos.XXXXXX") ||
	die "could not create a staging directory under $OUTPUT"
AUDIT="$STAGE/audit"
mkdir "$AUDIT"

CONTENTS="$STAGE/$APP_NAME.app/Contents"
MACOS="$CONTENTS/MacOS"
BINARY="$MACOS/$APP_NAME"
RESOURCES="$CONTENTS/Resources"
NOTICES="$RESOURCES/ThirdPartyNotices"
mkdir -p "$MACOS" "$NOTICES"

printf 'Building %s %s darwin/arm64\n' "$APP_NAME" "$VERSION"
(
	cd "$ROOT"
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		"$GO" build -mod=readonly -trimpath -buildvcs=false \
		-tags=mygo_noinspector -ldflags='-s -w' \
		-o "$BINARY" ./cmd/pdmview
)
chmod 755 "$BINARY"

sh "$ROOT/scripts/package-notices.sh" macos "$NOTICES" "$BINARY" "$AUDIT"
cp "$ICON_SOURCE" "$RESOURCES/$ICON_NAME"
cp "$LICENSE_SOURCE" "$RESOURCES/LICENSE"

cat >"$CONTENTS/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleDisplayName</key>
	<string>$APP_NAME</string>
	<key>CFBundleExecutable</key>
	<string>$APP_NAME</string>
	<key>CFBundleIconFile</key>
	<string>$ICON_NAME</string>
	<key>CFBundleIdentifier</key>
	<string>com.pdmview.app</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleLocalizations</key>
	<array>
		<string>en</string>
		<string>zh-Hans</string>
	</array>
	<key>CFBundleName</key>
	<string>$APP_NAME</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>$VERSION</string>
	<key>CFBundleVersion</key>
	<string>$VERSION</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.productivity</string>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
	<key>NSSupportsAutomaticGraphicsSwitching</key>
	<true/>
</dict>
</plist>
EOF
printf 'APPL????' >"$CONTENTS/PkgInfo"
plutil -lint "$CONTENTS/Info.plist" >/dev/null

[ "$(plutil -extract CFBundleIdentifier raw -o - "$CONTENTS/Info.plist")" = com.pdmview.app ] ||
	die "unexpected bundle identifier"
[ "$(plutil -extract CFBundleShortVersionString raw -o - "$CONTENTS/Info.plist")" = "$VERSION" ] ||
	die "unexpected bundle version"
[ "$(plutil -extract CFBundleIconFile raw -o - "$CONTENTS/Info.plist")" = "$ICON_NAME" ] ||
	die "unexpected bundle icon"

{
	cat <<'EOF'
Contents/Info.plist
Contents/MacOS/PdmView
Contents/PkgInfo
Contents/Resources/LICENSE
EOF
	printf 'Contents/Resources/%s\n' "$ICON_NAME"
	while IFS= read -r name; do
		printf 'Contents/Resources/ThirdPartyNotices/%s\n' "$name"
	done <"$AUDIT/notices-files.txt"
} >"$AUDIT/manifest.txt"
(
	cd "$STAGE/$APP_NAME.app"
	find . -type f | sed 's#^\./##' | LC_ALL=C sort >"$AUDIT/actual.txt"
)
LC_ALL=C sort "$AUDIT/manifest.txt" >"$AUDIT/manifest.sorted.txt"
cmp -s "$AUDIT/manifest.sorted.txt" "$AUDIT/actual.txt" ||
	die "app bundle does not match the packaging manifest"
if [ -n "$(find "$STAGE/$APP_NAME.app" ! -type d ! -type f -print)" ]; then
	die "app bundle contains non-regular files"
fi

ARCHS=$(lipo -archs "$BINARY")
[ "$ARCHS" = arm64 ] || die "unexpected Mach-O architecture: $ARCHS"
if otool -L "$BINARY" | grep -q 'WebKit'; then
	die "unexpected WebKit dependency"
fi
MINOS=$(xcrun vtool -show-build "$BINARY" | awk '$1 == "minos" { print $2; exit }')
[ "$MINOS" = 13.0 ] || die "unexpected Mach-O minos: ${MINOS:-unknown}"
[ "$(plutil -extract LSMinimumSystemVersion raw -o - "$CONTENTS/Info.plist")" = "$MINOS" ] ||
	die "Info.plist LSMinimumSystemVersion does not match the Mach-O minos"

codesign --force --sign - "$STAGE/$APP_NAME.app"
codesign --verify --deep --strict "$STAGE/$APP_NAME.app"
SIGNATURE=$(codesign -dv --verbose=4 "$STAGE/$APP_NAME.app" 2>&1 |
	sed -n 's/^Signature=//p' | head -1)
printf 'Signature: %s (ad hoc; no Developer ID, no notarization)\n' "${SIGNATURE:-unknown}"

/usr/bin/ditto -c -k --norsrc --keepParent \
	"$STAGE/$APP_NAME.app" "$STAGE/$ZIP_NAME"
unzip -t "$STAGE/$ZIP_NAME" >"$AUDIT/unzip-test.txt" ||
	die "zip integrity check failed"
unzip -Z1 "$STAGE/$ZIP_NAME" >"$AUDIT/zip-entries.txt"
if grep -Eq '(^|/)__MACOSX(/|$)|(^|/)\._' "$AUDIT/zip-entries.txt"; then
	die "zip contains AppleDouble metadata"
fi
zipinfo -l "$STAGE/$ZIP_NAME" |
	awk '$1 ~ /^[-dl][rwxStTs-]{9}$/ { print substr($1,1,1) "\t" $NF }' >"$AUDIT/zip-types.txt"
if awk -F '\t' '$1 != "-" && $1 != "d" { found = 1 } END { exit found ? 0 : 1 }' "$AUDIT/zip-types.txt"; then
	die "zip contains symlinks or special files"
fi
awk -F '\t' '$1 == "-" { print $2 }' "$AUDIT/zip-types.txt" |
	LC_ALL=C sort >"$AUDIT/zip-regular.txt"
{
	cat <<'EOF'
PdmView.app/Contents/Info.plist
PdmView.app/Contents/MacOS/PdmView
PdmView.app/Contents/PkgInfo
PdmView.app/Contents/_CodeSignature/CodeResources
PdmView.app/Contents/Resources/LICENSE
EOF
	printf 'PdmView.app/Contents/Resources/%s\n' "$ICON_NAME"
	while IFS= read -r name; do
		printf 'PdmView.app/Contents/Resources/ThirdPartyNotices/%s\n' "$name"
	done <"$AUDIT/notices-files.txt"
} >"$AUDIT/zip-expected.txt"
LC_ALL=C sort "$AUDIT/zip-expected.txt" >"$AUDIT/zip-expected.sorted.txt"
cmp -s "$AUDIT/zip-expected.sorted.txt" "$AUDIT/zip-regular.txt" ||
	die "zip regular files do not match the exact whitelist"
unzip -p "$STAGE/$ZIP_NAME" "PdmView.app/Contents/Resources/LICENSE" |
	cmp -s - "$LICENSE_SOURCE" ||
	die "zip LICENSE does not match the repository license"
cat >"$AUDIT/zip-expected-dirs.txt" <<'EOF'
PdmView.app/
PdmView.app/Contents/
PdmView.app/Contents/MacOS/
PdmView.app/Contents/Resources/
PdmView.app/Contents/Resources/ThirdPartyNotices/
PdmView.app/Contents/_CodeSignature/
EOF
awk -F '\t' '$1 == "d" { print $2 }' "$AUDIT/zip-types.txt" |
	LC_ALL=C sort >"$AUDIT/zip-dirs.txt"
LC_ALL=C sort "$AUDIT/zip-expected-dirs.txt" >"$AUDIT/zip-expected-dirs.sorted.txt"
cmp -s "$AUDIT/zip-expected-dirs.sorted.txt" "$AUDIT/zip-dirs.txt" ||
	die "zip directory entries do not match the packaging manifest"
unzip -p "$STAGE/$ZIP_NAME" "PdmView.app/Contents/Info.plist" |
	cmp -s - "$CONTENTS/Info.plist" ||
	die "zip Info.plist does not match the verified app bundle"
ARCHIVE_ICON="$AUDIT/archive-$ICON_NAME"
unzip -p "$STAGE/$ZIP_NAME" "PdmView.app/Contents/Resources/$ICON_NAME" >"$ARCHIVE_ICON" ||
	die "zip icon could not be read"
[ -s "$ARCHIVE_ICON" ] || die "zip icon is empty"
cmp -s "$ICON_SOURCE" "$ARCHIVE_ICON" ||
	die "zip icon does not match the repository icon"
ICON_HEADER=$(od -An -tx1 -N4 "$ARCHIVE_ICON" | tr -d ' \n')
[ "$ICON_HEADER" = 69636e73 ] || die "zip icon has no ICNS signature"
ICON_SIZE=$(wc -c <"$ARCHIVE_ICON" | tr -d ' ')
ICON_HEADER_SIZE=$(od -An -tu1 -j4 -N4 "$ARCHIVE_ICON" |
	awk 'NF == 4 { print $1 * 16777216 + $2 * 65536 + $3 * 256 + $4 }')
[ "$ICON_HEADER_SIZE" = "$ICON_SIZE" ] ||
	die "zip icon ICNS length does not match its file size"
ICON_TYPE=$(file -b "$ARCHIVE_ICON")
case "$ICON_TYPE" in
*"Mac OS X icon"*) ;;
*) die "zip icon has unexpected type: $ICON_TYPE" ;;
esac
{
	printf 'type: %s\n' "$ICON_TYPE"
	printf 'signature: %s\n' "$ICON_HEADER"
	printf 'bytes: %s\n' "$ICON_SIZE"
	printf 'source-match: yes\n'
} >"$AUDIT/icon.txt"

[ ! -e "$APP" ] || die "refusing to overwrite existing $APP"
[ ! -e "$ZIP" ] || die "refusing to overwrite existing $ZIP"
mv "$STAGE/$APP_NAME.app" "$APP"
[ -d "$APP" ] && [ ! -e "$STAGE/$APP_NAME.app" ] ||
	die "could not install app without overwriting an existing file"
mv "$STAGE/$ZIP_NAME" "$ZIP"
[ -f "$ZIP" ] && [ ! -e "$STAGE/$ZIP_NAME" ] ||
	die "could not install zip without overwriting an existing file"
mv "$AUDIT" "$OUTPUT/audit"
rmdir "$STAGE" 2>/dev/null || true

printf 'App: %s\nZip: %s\n' "$APP" "$ZIP"
printf 'Audit: %s\n' "$OUTPUT/audit"
printf 'Files: %s\n' "$(wc -l <"$OUTPUT/audit/manifest.txt" | tr -d ' ') manifest files plus ad-hoc signature"
printf 'Icon: %s (ICNS, %s bytes, matches source)\n' "$ICON_NAME" "$ICON_SIZE"
