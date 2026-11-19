#!/usr/bin/env bash
# 在 macOS 上构建 shared 的 iOS framework 并打包成 XCFramework。
#
# 用法：bash iosApp/scripts/build_shared.sh
# 产物：iosApp/Frameworks/McChainShared.xcframework（真机 arm64 + 模拟器 arm64）
#
# 说明：
#  - Kotlin/Native 的 iOS 交叉编译只能在 macOS（Xcode Command Line Tools）上执行；
#  - Windows 侧只能验证 commonMain 的 metadata 编译与 JVM 单测（签名/编码/派生逐字节一致）；
#  - 模拟器只产出 arm64 切片（Apple Silicon）；Intel Mac 可追加 iosX64 目标后再链一次。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"   # mc-miner
IOS_APP="$(cd "$(dirname "$0")/.." && pwd)"
GRADLE="$ROOT/gradlew"

echo "==> 构建 iOS framework（release）"
"$GRADLE" -p "$ROOT" \
  :shared:linkReleaseFrameworkIosArm64 \
  :shared:linkReleaseFrameworkIosSimulatorArm64

ARM64_FW="$ROOT/shared/build/bin/iosArm64/releaseFramework/McChainShared.framework"
SIM_FW="$ROOT/shared/build/bin/iosSimulatorArm64/releaseFramework/McChainShared.framework"
OUT="$IOS_APP/Frameworks/McChainShared.xcframework"

for fw in "$ARM64_FW" "$SIM_FW"; do
  if [ ! -d "$fw" ]; then
    echo "缺少 framework：$fw" >&2
    exit 1
  fi
done

echo "==> 打包 XCFramework"
rm -rf "$OUT"
mkdir -p "$(dirname "$OUT")"
xcodebuild -create-xcframework \
  -framework "$ARM64_FW" \
  -framework "$SIM_FW" \
  -output "$OUT"

echo "==> 完成：$OUT"
echo "下一步：cd $IOS_APP && xcodegen generate && open McMiner.xcodeproj"
