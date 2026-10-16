#!/usr/bin/env bash
# Builds the plugin bundle: dist/<id>-<version>.tar.gz, ready to upload in
# System Console → Plugins → Plugin Management. Needs Go (brew install go).
#
#   ./build.sh                 all platforms (about 35 MB)
#   ./build.sh linux-amd64     one platform only (smaller), e.g. for a typical Linux server
set -euo pipefail
cd "$(dirname "$0")"

ID=$(sed -n 's/^    "id": "\(.*\)",$/\1/p' plugin.json)
VERSION=$(sed -n 's/^    "version": "\(.*\)",$/\1/p' plugin.json)
if [ -z "$ID" ] || [ -z "$VERSION" ]; then
    echo "Could not read id/version from plugin.json" >&2
    exit 1
fi

echo "Testing…"
go vet ./...
go test ./server/ -count=1
node --check webapp/main.js

OUT="dist/$ID"
rm -rf dist
mkdir -p "$OUT/server/dist" "$OUT/webapp/dist" "$OUT/assets"

ALL="linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64"
TARGETS="${*:-$ALL}"
SUFFIX=""
[ $# -gt 0 ] && SUFFIX="-$(echo "$TARGETS" | tr ' ' '+')"

echo "Building $ID $VERSION for: $TARGETS"
for target in $TARGETS; do
    case " $ALL " in *" $target "*) ;; *) echo "Unknown platform: $target" >&2; exit 1 ;; esac
    os=${target%-*}
    arch=${target#*-}
    ext=""
    [ "$os" = "windows" ] && ext=".exe"
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
        -o "$OUT/server/dist/plugin-$os-$arch$ext" ./server
done

# List only the platforms that were built.
node -e '
const fs = require("fs");
const m = JSON.parse(fs.readFileSync("plugin.json", "utf8"));
const keep = process.argv[1].split(" ");
for (const k of Object.keys(m.server.executables)) if (!keep.includes(k)) delete m.server.executables[k];
fs.writeFileSync(process.argv[2], JSON.stringify(m, null, 4) + "\n");
' "$TARGETS" "$OUT/plugin.json"
cp webapp/main.js "$OUT/webapp/dist/main.js"
cp assets/icon.svg assets/bot.png "$OUT/assets/"
cp README.md LICENSE "$OUT/"

FILE="dist/$ID-$VERSION$SUFFIX.tar.gz"
# macOS tar otherwise adds hidden "._" files (extended attributes); a "._<id>" next to the
# plugin folder makes Mattermost answer "Unable to find manifest for extracted plugin".
COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata -C dist -czf "$FILE" "$ID" 2>/dev/null ||
    COPYFILE_DISABLE=1 tar -C dist -czf "$FILE" "$ID"

# Check the archive the way Mattermost reads it: one top folder, plugin.json inside, no "._" files.
node -e '
const zlib = require("zlib"), fs = require("fs");
const buf = zlib.gunzipSync(fs.readFileSync(process.argv[1]));
const names = [];
for (let off = 0; off + 512 <= buf.length;) {
    const name = buf.toString("utf8", off, off + 100).replace(/\0.*$/s, "");
    if (!name) break;
    const prefix = buf.toString("utf8", off + 345, off + 500).replace(/\0.*$/s, "");
    const size = parseInt(buf.toString("utf8", off + 124, off + 136).replace(/\0.*$/s, "").trim() || "0", 8);
    names.push(prefix ? prefix + "/" + name : name);
    off += 512 + Math.ceil(size / 512) * 512;
}
const id = process.argv[2];
const bad = names.filter((n) => n.split("/").some((p) => p.startsWith("._")) || !n.startsWith(id + "/") && n !== id + "/");
if (bad.length || !names.includes(id + "/plugin.json")) {
    console.error("Archive check failed:", bad.length ? bad : "no plugin.json");
    process.exit(1);
}
' "$FILE" "$ID"

echo "Done: $FILE ($(du -h "$FILE" | cut -f1))"
