#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
VERSION=${VERSION-0.1.0}
APP_NAME=PdmView
PACKAGE_NAME=pdmview
MAINTAINER='PdmView Maintainers <gloryhui@users.noreply.github.com>'
HOMEPAGE=https://github.com/gloryhui/PdmView
ICON_SOURCE="$ROOT/assets/$APP_NAME.png"
LICENSE_SOURCE="$ROOT/LICENSE"
OUT="$ROOT/.out/release"

die() {
	printf 'package-linux: %s\n' "$*" >&2
	exit 1
}

case "$VERSION" in
'' | *[!0-9.]*)
	die "VERSION must be a numeric version in x.y.z form (digits and dots only, no newlines)"
	;;
esac
printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ||
	die "VERSION must be a numeric version in x.y.z form (got: $VERSION)"

command -v "$GO" >/dev/null 2>&1 || die "Go command not found: $GO"
command -v dpkg-deb >/dev/null 2>&1 ||
	die "dpkg-deb is required to build and verify the .deb (use an Ubuntu/Debian runner)"
dpkg-deb --help 2>&1 | grep -Fq -- '--root-owner-group' ||
	die "dpkg-deb does not support --root-owner-group"
command -v file >/dev/null 2>&1 || die "file command not found"
command -v shasum >/dev/null 2>&1 || die "shasum command not found"
[ -s "$ICON_SOURCE" ] || die "Linux PNG icon missing or empty: $ICON_SOURCE"
[ -s "$LICENSE_SOURCE" ] || die "repository license missing or empty: $LICENSE_SOURCE"

VERSION_DIR="$OUT/$VERSION"
BASE_OUTPUT="$VERSION_DIR/linux-amd64"
DEB_NAME="$APP_NAME-$VERSION-linux-amd64.deb"
mkdir -p "$VERSION_DIR"
if mkdir "$BASE_OUTPUT" 2>/dev/null; then
	OUTPUT="$BASE_OUTPUT"
else
	OUTPUT=$(mktemp -d "$VERSION_DIR/linux-amd64.XXXXXX") ||
		die "could not create a unique output directory under $VERSION_DIR"
fi

STAGE=
on_exit() {
	status=$?
	trap - EXIT
	if [ "$status" -ne 0 ] && [ -n "${STAGE:-}" ] && [ -d "$STAGE" ]; then
		printf 'package-linux: staging retained at %s\n' "$STAGE" >&2
	fi
	exit "$status"
}
trap on_exit EXIT

STAGE=$(mktemp -d "$OUTPUT/.package-linux.XXXXXX") ||
	die "could not create a staging directory under $OUTPUT"
AUDIT="$STAGE/audit"
DEBROOT="$STAGE/debroot"
CONTROL="$DEBROOT/DEBIAN"
BINARY="$STAGE/$APP_NAME"
BIN_TARGET="$DEBROOT/usr/bin/$PACKAGE_NAME"
DESKTOP_TARGET="$DEBROOT/usr/share/applications/$PACKAGE_NAME.desktop"
ICON_TARGET="$DEBROOT/usr/share/icons/hicolor/1024x1024/apps/$PACKAGE_NAME.png"
DOC_DIR="$DEBROOT/usr/share/doc/$PACKAGE_NAME"
NOTICES="$DOC_DIR/ThirdPartyNotices"
mkdir -p "$AUDIT" "$CONTROL" "$(dirname "$BIN_TARGET")" \
	"$(dirname "$DESKTOP_TARGET")" "$(dirname "$ICON_TARGET")" "$NOTICES"

printf 'Building %s %s linux/amd64\n' "$APP_NAME" "$VERSION"
(
	cd "$ROOT"
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		"$GO" build -mod=readonly -trimpath -buildvcs=false \
		-tags=mygo_noinspector -ldflags='-s -w' \
		-o "$BINARY" ./cmd/pdmview
)

FILE_INFO=$(file -b "$BINARY")
printf '%s\n' "$FILE_INFO" >"$AUDIT/file.txt"
case "$FILE_INFO" in
*'ELF 64-bit LSB executable, x86-64'*) ;;
*) die "unexpected ELF format: $FILE_INFO" ;;
esac
case "$FILE_INFO" in
*'dynamically linked'*) ;;
*) die "binary is not dynamically linked: $FILE_INFO" ;;
esac
case "$FILE_INFO" in
*stripped*) ;;
*) die "binary is not stripped: $FILE_INFO" ;;
esac

"$GO" version -m "$BINARY" >"$AUDIT/go-version-m.txt"
for setting in \
	'GOOS=linux' \
	'GOARCH=amd64' \
	'CGO_ENABLED=0' \
	'-trimpath=true'
do
	grep -Fq -- "$setting" "$AUDIT/go-version-m.txt" ||
		die "build metadata lacks $setting"
done
awk '$1 == "build" && $2 == "-tags=mygo_noinspector" { found = 1 } END { exit found ? 0 : 1 }' \
	"$AUDIT/go-version-m.txt" ||
	die "build metadata must contain exactly the tag set mygo_noinspector"
if grep -Fq 'vcs=' "$AUDIT/go-version-m.txt"; then
	die "build metadata contains VCS information"
fi

cp "$BINARY" "$BIN_TARGET"
chmod 755 "$BIN_TARGET"
cp "$ICON_SOURCE" "$ICON_TARGET"
chmod 644 "$ICON_TARGET"
cp "$LICENSE_SOURCE" "$DOC_DIR/LICENSE"
chmod 644 "$DOC_DIR/LICENSE"

cat >"$AUDIT/$PACKAGE_NAME.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=PdmView
Exec=pdmview %f
Icon=pdmview
Terminal=false
Categories=Development;Database;
EOF
cp "$AUDIT/$PACKAGE_NAME.desktop" "$DESKTOP_TARGET"
chmod 644 "$DESKTOP_TARGET"

sh "$ROOT/scripts/package-notices.sh" linux "$NOTICES" "$BINARY" "$AUDIT"

INSTALLED_SIZE=$(du -sk "$DEBROOT/usr" | awk '{ print $1 }')
cat >"$CONTROL/control" <<EOF
Package: $PACKAGE_NAME
Version: $VERSION
Section: devel
Priority: optional
Architecture: amd64
Installed-Size: $INSTALLED_SIZE
Maintainer: $MAINTAINER
Homepage: $HOMEPAGE
Depends: libc6 (>= 2.17), libgtk-3-0t64 | libgtk-3-0, libglib2.0-0t64 | libglib2.0-0, libcairo2, libgdk-pixbuf-2.0-0
Description: native PowerDesigner PDM model viewer
 PdmView opens local PowerDesigner PDM XML models, browses tables and
 diagrams, and previews or exports MySQL 8.0 schema SQL.
EOF
chmod 755 "$CONTROL"
chmod 644 "$CONTROL/control"

cat >"$AUDIT/data-paths-expected.txt" <<'EOF'
usr
usr/bin
usr/bin/pdmview
usr/share
usr/share/applications
usr/share/applications/pdmview.desktop
usr/share/doc
usr/share/doc/pdmview
usr/share/doc/pdmview/LICENSE
usr/share/doc/pdmview/ThirdPartyNotices
usr/share/icons
usr/share/icons/hicolor
usr/share/icons/hicolor/1024x1024
usr/share/icons/hicolor/1024x1024/apps
usr/share/icons/hicolor/1024x1024/apps/pdmview.png
EOF
while IFS= read -r name; do
	printf 'usr/share/doc/pdmview/ThirdPartyNotices/%s\n' "$name"
done <"$AUDIT/notices-files.txt" >>"$AUDIT/data-paths-expected.txt"
LC_ALL=C sort "$AUDIT/data-paths-expected.txt" >"$AUDIT/data-paths-expected.sorted.txt"
dpkg-deb --root-owner-group --build "$DEBROOT" "$STAGE/$DEB_NAME"
dpkg-deb --info "$STAGE/$DEB_NAME" >"$AUDIT/deb-info.txt"
dpkg-deb --contents "$STAGE/$DEB_NAME" >"$AUDIT/deb-contents.txt"
mkdir "$AUDIT/control" "$AUDIT/extracted"
dpkg-deb --control "$STAGE/$DEB_NAME" "$AUDIT/control"
dpkg-deb --extract "$STAGE/$DEB_NAME" "$AUDIT/extracted"

[ "$(dpkg-deb --show --showformat='${Package} ${Version} ${Architecture}' \
	"$STAGE/$DEB_NAME")" = "$PACKAGE_NAME $VERSION amd64" ] ||
	die "unexpected package identity metadata"
[ "$(dpkg-deb --field "$STAGE/$DEB_NAME" Depends)" = \
	'libc6 (>= 2.17), libgtk-3-0t64 | libgtk-3-0, libglib2.0-0t64 | libglib2.0-0, libcairo2, libgdk-pixbuf-2.0-0' ] ||
	die "unexpected package dependencies"
if awk '$2 != "root/root" { bad = 1 } END { exit bad ? 0 : 1 }' \
	"$AUDIT/deb-contents.txt"; then
	die ".deb contains files not owned by root/root"
fi

(
	cd "$AUDIT/control"
	find . -mindepth 1 | sed 's#^\./##' | LC_ALL=C sort
) >"$AUDIT/control-actual.txt"
printf 'control\n' >"$AUDIT/control-expected.txt"
cmp -s "$AUDIT/control-expected.txt" "$AUDIT/control-actual.txt" ||
	die "package contains unexpected maintainer scripts"

(
	cd "$AUDIT/extracted"
	find . -mindepth 1 | sed 's#^\./##' | LC_ALL=C sort
) >"$AUDIT/deb-paths-actual.txt"
cmp -s "$AUDIT/data-paths-expected.sorted.txt" "$AUDIT/deb-paths-actual.txt" ||
	die "extracted .deb paths do not match the exact path manifest"
if [ -n "$(find "$AUDIT/extracted" ! -type d ! -type f -print)" ]; then
	die "extracted .deb contains symlinks or special files"
fi

cmp -s "$LICENSE_SOURCE" "$AUDIT/extracted/usr/share/doc/$PACKAGE_NAME/LICENSE" ||
	die "packaged LICENSE differs from the repository license"
cmp -s "$ICON_SOURCE" \
	"$AUDIT/extracted/usr/share/icons/hicolor/1024x1024/apps/$PACKAGE_NAME.png" ||
	die "packaged icon differs from the repository icon"
cmp -s "$AUDIT/$PACKAGE_NAME.desktop" \
	"$AUDIT/extracted/usr/share/applications/$PACKAGE_NAME.desktop" ||
	die "packaged desktop launcher differs from the audited launcher"
cmp -s "$BIN_TARGET" "$AUDIT/extracted/usr/bin/$PACKAGE_NAME" ||
	die "packaged executable differs from the audited binary"
while IFS= read -r name; do
	cmp -s "$NOTICES/$name" \
		"$AUDIT/extracted/usr/share/doc/$PACKAGE_NAME/ThirdPartyNotices/$name" ||
		die "packaged third-party notice differs: $name"
done <"$AUDIT/notices-files.txt"
[ ! -x "$AUDIT/extracted/usr/share/applications/$PACKAGE_NAME.desktop" ] ||
	die "desktop launcher must not be executable"
[ ! -x "$AUDIT/extracted/usr/share/doc/$PACKAGE_NAME/LICENSE" ] ||
	die "LICENSE must not be executable"
[ -x "$AUDIT/extracted/usr/bin/$PACKAGE_NAME" ] ||
	die "packaged executable is not executable"

DEB_SHA=$(shasum -a 256 "$STAGE/$DEB_NAME" | awk '{ print $1 }')
NOTICE_COUNT=$(wc -l <"$AUDIT/notices-files.txt" | tr -d ' ')
{
	printf 'file: %s\n' "$FILE_INFO"
	printf 'deb-sha256: %s\n' "$DEB_SHA"
	printf 'notices: %s files\n' "$NOTICE_COUNT"
	printf 'build-tags: mygo_noinspector\n'
} >"$AUDIT/summary.txt"

DEB="$OUTPUT/$DEB_NAME"
[ ! -e "$DEB" ] || die "refusing to overwrite existing $DEB"
mv "$STAGE/$DEB_NAME" "$DEB"
[ -f "$DEB" ] && [ ! -e "$STAGE/$DEB_NAME" ] ||
	die "could not install .deb without overwriting an existing file"
mv "$AUDIT" "$OUTPUT/audit"
rmdir "$STAGE" 2>/dev/null || true

printf 'Deb: %s\n' "$DEB"
printf 'Audit: %s\n' "$OUTPUT/audit"
printf 'SHA-256 (deb): %s\n' "$DEB_SHA"
