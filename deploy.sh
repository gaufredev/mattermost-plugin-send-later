#!/usr/bin/env bash
# Installs a new version on the Mattermost server in one step: runs the tests, builds,
# uploads (replacing the old version), switches it on, and checks the new version is live.
#
#   MM_URL=https://chat.example.com ./deploy.sh                  (linux-amd64 build)
#   MM_URL=https://chat.example.com ARCH=linux-arm64 ./deploy.sh
#
# Put MM_URL=… (and ARCH=… if needed) in a file named .deploy.env next to this script to
# skip typing it; that file is not committed.
#
# Needs, once: a token of a system admin (or of a bot with the system admin role) saved in
# the macOS Keychain. Mattermost: 관리자 도구 → 통합 → 봇 계정 → (bot) → 새 토큰 만들기,
# or 프로필 → 보안 → 개인 액세스 토큰. Then, in Terminal (it asks for the token, hidden):
#   security add-generic-password -a "$USER" -s mattermost-send-later-deploy -w
# It never updates by itself: someone runs this command.
set -euo pipefail
cd "$(dirname "$0")"

# Values given on the command line win over .deploy.env.
cli_url="${MM_URL:-}" cli_arch="${ARCH:-}" cli_token="${MM_TOKEN:-}"
if [ -f .deploy.env ]; then
    # shellcheck disable=SC1091
    . ./.deploy.env
fi
MM_URL="${cli_url:-${MM_URL:-}}"
ARCH="${cli_arch:-${ARCH:-}}"
MM_TOKEN="${cli_token:-${MM_TOKEN:-}}"
MM_URL="${MM_URL%/}"
if [ -z "$MM_URL" ]; then
    echo "Which server? Run as  MM_URL=https://chat.example.com ./deploy.sh  or put MM_URL=… in .deploy.env" >&2
    exit 1
fi
ARCH="${ARCH:-linux-amd64}"
KEYCHAIN_ITEM="mattermost-send-later-deploy"

TOKEN="${MM_TOKEN:-}" # optional, e.g. for automation; otherwise the Keychain
if [ -z "$TOKEN" ] && ! TOKEN=$(security find-generic-password -s "$KEYCHAIN_ITEM" -w 2>/dev/null); then
    TOKEN=""
fi
if [ -z "$TOKEN" ]; then
    echo "No token in the Keychain yet. Create one in Mattermost, then run:" >&2
    echo "  security add-generic-password -a \"\$USER\" -s $KEYCHAIN_ITEM -w" >&2
    exit 1
fi

TMP=$(mktemp -t send-later-deploy)
trap 'rm -f "$TMP"' EXIT

ID=$(sed -n 's/^    "id": "\(.*\)",$/\1/p' plugin.json)
VERSION=$(sed -n 's/^    "version": "\(.*\)",$/\1/p' plugin.json)

# The token goes to curl on stdin, so it never shows in the process list.
api() {
    local method=$1 path=$2
    shift 2
    printf 'Authorization: Bearer %s\n' "$TOKEN" |
        curl -sS -o $TMP -w '%{http_code}' -H @- -X "$method" "$@" "$MM_URL/api/v4$path"
}

live_version() {
    curl -sS "$MM_URL/api/v4/plugins/webapp" |
        node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const p=JSON.parse(s).find(x=>x.id===process.argv[1]);console.log(p?p.version:"")})' "$ID"
}

echo "Now live on $MM_URL: $(live_version || true)"
./build.sh "$ARCH"
FILE="dist/$ID-$VERSION-$ARCH.tar.gz"

echo "Uploading $FILE…"
code=$(api POST /plugins -F "plugin=@$FILE" -F "force=true")
if [ "$code" != "201" ] && [ "$code" != "200" ]; then
    echo "Upload failed (HTTP $code): $(cat $TMP)" >&2
    exit 1
fi

echo "Switching it on…"
code=$(api POST "/plugins/$ID/enable")
if [ "$code" != "200" ]; then
    echo "Enable failed (HTTP $code): $(cat $TMP)" >&2
    exit 1
fi

for _ in $(seq 1 30); do
    if [ "$(live_version)" = "$VERSION" ]; then
        echo "Done: $ID $VERSION is live on $MM_URL. Reload Mattermost (Cmd+R) to get the new screens."
        exit 0
    fi
    sleep 1
done
echo "Uploaded and switched on, but $MM_URL still reports version $(live_version). Check 관리자 도구 → 플러그인." >&2
exit 1
