#!/usr/bin/env bash
# Builds the Career Assessment 107 Android app:
#   1. the React UI          -> api/webdist   (embedded into the Go library)
#   2. the Go runtime (AAR)  -> android/app/libs/career107.aar  (gomobile bind)
#   3. the APKs              -> dist/
#
# Requirements: Go 1.26+, Node 20+, JDK 17, Android SDK (platform 36) and NDK.
# Usage: scripts/build-apk.sh [debug|release|all]   (default: all)
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT=$(pwd)
VARIANT=${1:-all}

export ANDROID_HOME=${ANDROID_HOME:-$HOME/Android/Sdk}
if [ -z "${ANDROID_NDK_HOME:-}" ]; then
  ANDROID_NDK_HOME=$(ls -d "$ANDROID_HOME"/ndk/* 2>/dev/null | sort -V | tail -1)
  export ANDROID_NDK_HOME
fi
[ -d "$ANDROID_NDK_HOME" ] || { echo "Android NDK not found; set ANDROID_NDK_HOME" >&2; exit 1; }

GOBIN_DIR=$(go env GOBIN); [ -n "$GOBIN_DIR" ] || GOBIN_DIR="$(go env GOPATH)/bin"
export PATH="$GOBIN_DIR:$PATH"
if ! command -v gomobile >/dev/null; then
  echo "==> installing gomobile"
  go install golang.org/x/mobile/cmd/gomobile@latest
  go install golang.org/x/mobile/cmd/gobind@latest
  gomobile init
fi

echo "==> testing Go packages"
go test ./...

echo "==> building web UI"
(cd web && { [ -d node_modules ] || npm ci; } && npm run build)

echo "==> binding Go runtime for Android"
mkdir -p android/app/libs
gomobile bind -target=android/arm64,android/arm,android/amd64 -androidapi 24 \
  -javapkg com.career107 -o android/app/libs/career107.aar ./mobile

echo "==> building APK(s)"
echo "sdk.dir=$ANDROID_HOME" > android/local.properties
cd android
case "$VARIANT" in
  debug) ./gradlew assembleDebug ;;
  release) ./gradlew assembleRelease ;;
  all) ./gradlew assembleDebug assembleRelease ;;
  *) echo "unknown variant $VARIANT" >&2; exit 1 ;;
esac
cd "$ROOT"

mkdir -p dist
for apk in android/app/build/outputs/apk/*/*.apk; do
  cp "$apk" dist/
done
ls -la dist
