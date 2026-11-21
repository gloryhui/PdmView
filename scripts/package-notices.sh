#!/bin/sh
set -eu

GO=${GO:-go}

die() {
	printf 'package-notices: %s\n' "$*" >&2
	exit 1
}

[ "$#" -eq 4 ] || die "usage: package-notices.sh macos|windows DEST BINARY AUDIT"
TARGET=$1
DEST=$2
BINARY=$3
AUDIT=$4

case "$TARGET" in
macos | windows) ;;
*) die "target must be macos or windows" ;;
esac
[ -f "$BINARY" ] || die "binary not found: $BINARY"
[ -d "$DEST" ] || die "notices directory not found: $DEST"
[ -d "$AUDIT" ] || die "audit directory not found: $AUDIT"
command -v "$GO" >/dev/null 2>&1 || die "Go command not found: $GO"

GOROOT=$("$GO" env GOROOT)
GOVERSION=$("$GO" env GOVERSION)
"$GO" version -m "$BINARY" >"$AUDIT/notices-go-version-m.txt"
BIN_GOVERSION=$(awk 'NR == 1 { print $NF }' "$AUDIT/notices-go-version-m.txt")
[ "$BIN_GOVERSION" = "$GOVERSION" ] ||
	die "binary Go version $BIN_GOVERSION does not match $GOVERSION"

DEPS="$AUDIT/notices-deps.txt"
awk '$1 == "dep" { print $2 "\t" $3 }' "$AUDIT/notices-go-version-m.txt" |
	LC_ALL=C sort >"$DEPS"
[ -s "$DEPS" ] || die "binary has no linked modules"

EXPECTED="$AUDIT/notices-expected-modules.txt"
cat >"$EXPECTED" <<'EOF'
github.com/ebitengine/purego
github.com/egoist/mygo
github.com/go-text/typesetting
golang.org/x/image
golang.org/x/text
EOF
if [ "$TARGET" = windows ]; then
	printf 'golang.org/x/net\n' >>"$EXPECTED"
fi
LC_ALL=C sort "$EXPECTED" >"$EXPECTED.sorted"
awk -F '\t' '{ print $1 }' "$DEPS" | LC_ALL=C sort -u >"$AUDIT/notices-actual-modules.txt"
cmp -s "$EXPECTED.sorted" "$AUDIT/notices-actual-modules.txt" ||
	die "linked modules do not match the $TARGET notice set"
mv "$EXPECTED.sorted" "$EXPECTED"

MANIFEST="$AUDIT/notices-manifest.tsv"
FILES="$AUDIT/notices-files.unsorted.txt"
MODULE_DIRS="$AUDIT/notices-modules.txt"
: >"$MANIFEST"
: >"$FILES"
: >"$MODULE_DIRS"

record() {
	module=$1
	version=$2
	source=$3
	name=$4
	path=$5
	[ -f "$path" ] && [ -s "$path" ] || die "missing license: $path"
	cp "$path" "$DEST/$name"
	chmod 644 "$DEST/$name"
	cmp -s "$path" "$DEST/$name" || die "copied license differs: $name"
	sha=$(shasum -a 256 "$DEST/$name" | awk '{ print $1 }')
	printf '%s\t%s\t%s\t%s\t%s\n' \
		"$module" "$version" "$source" "$name" "$sha" >>"$MANIFEST"
	printf '%s\n' "$name" >>"$FILES"
}

record Go "$GOVERSION" LICENSE Go-LICENSE "$GOROOT/LICENSE"
record Go "$GOVERSION" PATENTS Go-PATENTS "$GOROOT/PATENTS"

while IFS="$(printf '\t')" read -r module version; do
	info=$(env GOPROXY=off GOFLAGS= "$GO" list -mod=readonly -m \
		-f '{{.Version}} {{.Dir}}' "$module") ||
		die "go list failed for $module"
	mod_version=${info%% *}
	mod_dir=${info#* }
	[ "$mod_version" = "$version" ] ||
		die "version mismatch for $module: binary=$version cache=$mod_version"
	[ -n "$mod_dir" ] && [ -d "$mod_dir" ] || die "module cache missing for $module"
	printf '%s\t%s\t%s\n' "$module" "$version" "$mod_dir" >>"$MODULE_DIRS"
	case "$module" in
	github.com/egoist/mygo)
		record "$module" "$version" LICENSE MyGo-LICENSE "$mod_dir/LICENSE"
		record "$module" "$version" internal/shader/LICENSE \
			MyGo-shader-LICENSE "$mod_dir/internal/shader/LICENSE"
		;;
	github.com/ebitengine/purego)
		record "$module" "$version" LICENSE purego-LICENSE "$mod_dir/LICENSE"
		;;
	github.com/go-text/typesetting)
		record "$module" "$version" LICENSE \
			go-text-typesetting-LICENSE "$mod_dir/LICENSE"
		record "$module" "$version" harfbuzz/LICENSE \
			go-text-typesetting-harfbuzz-LICENSE "$mod_dir/harfbuzz/LICENSE"
		;;
	golang.org/x/image)
		record "$module" "$version" LICENSE x-image-LICENSE "$mod_dir/LICENSE"
		record "$module" "$version" PATENTS x-image-PATENTS "$mod_dir/PATENTS"
		;;
	golang.org/x/net)
		record "$module" "$version" LICENSE x-net-LICENSE "$mod_dir/LICENSE"
		record "$module" "$version" PATENTS x-net-PATENTS "$mod_dir/PATENTS"
		;;
	golang.org/x/text)
		record "$module" "$version" LICENSE x-text-LICENSE "$mod_dir/LICENSE"
		record "$module" "$version" PATENTS x-text-PATENTS "$mod_dir/PATENTS"
		;;
	*)
		die "no notice rules for linked module $module"
		;;
	esac
done <"$DEPS"

{
	printf '# PdmView third-party notices\n'
	printf '# module\tversion\tsource\tfile\tsha256\n'
	cat "$MANIFEST"
} >"$DEST/modules.txt"
printf 'modules.txt\n' >>"$FILES"

LC_ALL=C sort "$FILES" >"$AUDIT/notices-files.txt"
(
	cd "$DEST"
	find . -type f | sed 's#^\./##' | LC_ALL=C sort
) >"$AUDIT/notices-actual.txt"
cmp -s "$AUDIT/notices-files.txt" "$AUDIT/notices-actual.txt" ||
	die "notices directory does not match the generated file list"
if [ -n "$(find "$DEST" ! -type d ! -type f -print)" ]; then
	die "notices directory contains non-regular files"
fi

printf 'Notices: %s files for %s (%s)\n' \
	"$(wc -l <"$AUDIT/notices-files.txt" | tr -d ' ')" "$TARGET" "$GOVERSION"
