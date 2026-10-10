#!/bin/sh
# Build Linux and Windows (mingw64) versions of busybox-w32 on Linux.
# Requires: gcc, make, x86_64-w64-mingw32-gcc (for the Windows build).
#
# Usage: ./build.sh [linux] [win64]
#   default: build both

set -e

cd "$(dirname "$0")"
JOBS=${JOBS:-$(nproc)}
BUILD_LINUX=0
BUILD_WIN=0
[ $# -eq 0 ] && BUILD_LINUX=1 && BUILD_WIN=1
for arg in "$@"; do
    case "$arg" in
        linux) BUILD_LINUX=1 ;;
        win64) BUILD_WIN=1 ;;
        *) echo "unknown target: $arg (expected: linux, win64)" >&2; exit 1 ;;
    esac
done

# Out-of-tree (O=) builds require a clean source tree.
# Run mrproper if there are generated leftovers in the source dir.
if [ -f .config ] || [ -f include/BB_VER.h ]; then
    echo "== cleaning source tree (make mrproper) =="
    make mrproper
fi

if [ "$BUILD_LINUX" = 1 ]; then
    echo "== building Linux version =="
    mkdir -p build-linux
    make O=build-linux defconfig
    make O=build-linux -j"$JOBS"
    echo "Linux binary: build-linux/busybox"
fi

if [ "$BUILD_WIN" = 1 ]; then
    echo "== building Windows (x86_64) version =="
    mkdir -p build-win64
    make O=build-win64 mingw64_defconfig
    make O=build-win64 CROSS_COMPILE=x86_64-w64-mingw32- -j"$JOBS"
    echo "Windows binary: build-win64/busybox.exe"
fi

echo "== done =="
