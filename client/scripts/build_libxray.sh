#!/usr/bin/env bash
# Build libXray native bindings for RFPlay client.
#
# Android: produces an .aar copied into android/app/libs/libXray.aar
# Apple:   produces .xcframework copied into ios/LibXray.xcframework and
#          macos/LibXray.xcframework
#
# Prerequisites:
#   - Go toolchain
#   - Android NDK + ANDROID_HOME / ANDROID_SDK_ROOT
#   - Xcode + iOS SDK (for Apple targets)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${LIBXRAY_BUILD_DIR:-$ROOT/third_party/libXray}"
TAG="${LIBXRAY_TAG:-v0.8.6}"

log() { printf '\033[1;34m[libXray]\033[0m %s\n' "$*"; }

if [ ! -d "$BUILD_DIR" ]; then
  log "cloning XTLS/libXray@${TAG} into $BUILD_DIR"
  git clone --depth 1 --branch "$TAG" https://github.com/XTLS/libXray.git "$BUILD_DIR"
else
  log "using existing libXray checkout at $BUILD_DIR"
fi

pushd "$BUILD_DIR" >/dev/null

case "${1:-android}" in
  android)
    log "building Android .aar"
    python3 build/main.py android
    mkdir -p "$ROOT/android/app/libs"
    cp -f android/libXray.aar "$ROOT/android/app/libs/libXray.aar" 2>/dev/null \
      || cp -f build/android/libXray.aar "$ROOT/android/app/libs/libXray.aar" 2>/dev/null \
      || (echo "could not find built .aar under $BUILD_DIR/android" && exit 1)
    log "Android .aar installed at android/app/libs/libXray.aar"
    ;;

  apple)
    log "building Apple frameworks (gomobile)"
    python3 build/main.py apple gomobile
    if [ -d "apple/LibXray.xcframework" ]; then
      SRC="apple/LibXray.xcframework"
    elif [ -d "build/apple/LibXray.xcframework" ]; then
      SRC="build/apple/LibXray.xcframework"
    else
      echo "could not find built xcframework"; exit 1
    fi
    rm -rf "$ROOT/ios/LibXray.xcframework" "$ROOT/macos/LibXray.xcframework"
    cp -R "$SRC" "$ROOT/ios/LibXray.xcframework"
    cp -R "$SRC" "$ROOT/macos/LibXray.xcframework"
    log "Apple xcframework installed into ios/ and macos/"
    ;;

  windows)
    log "building Windows libxray.dll (amd64)"
    CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
      go build -buildmode=c-shared -o "$ROOT/windows/libs/libxray.dll" ./lib
    log "Windows libxray.dll installed at windows/libs/libxray.dll"
    ;;

  linux)
    log "building Linux libxray.so (amd64)"
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
      go build -buildmode=c-shared -o "$ROOT/linux/libs/libxray.so" ./lib
    log "Linux libxray.so installed at linux/libs/libxray.so"
    ;;

  macos-dylib)
    log "building macOS libxray.dylib (arm64)"
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
      go build -buildmode=c-shared -o "$ROOT/macos/libs/libxray.dylib" ./lib
    log "macOS libxray.dylib installed at macos/libs/libxray.dylib"
    ;;

  *)
    echo "usage: $0 [android|apple|windows|linux|macos-dylib]"
    exit 1
    ;;
esac

popd >/dev/null
log "done"
