plugins {
    kotlin("multiplatform")
}

kotlin {
    jvmToolchain(17)

    // Android 直接消费 jvm 目标。
    jvm {
        compilations.all {
            kotlinOptions.jvmTarget = "17"
        }
    }

    // Phase 5：iOS 目标（XCFramework 供 SwiftUI 工程消费）。
    // 注意：Kotlin/Native 的 iOS 编译/链接只能在 macOS 主机执行；
    // Windows/Linux 上这些目标的 task 会失败，但 commonMain 的 metadata
    // 编译可在任意主机验证跨平台兼容性（CI 用 :shared:allTests 分机跑）。
    //
    // 签名架构：签名器已下沉到 commonMain（PureSecp256k1Signer，纯 Kotlin secp256k1 +
    // RFC6979 + low-S），与 Android 共用同一份实现；iOS 侧直接
    // `McWallet.signer()` 取到 TxSigner 传入 DirectTxBuilder.build，无需第三方加密库。
    // 注意 iOS 的 Secure Enclave 只支持 P-256，无法保存 secp256k1 私钥，
    // 因此私钥由 Keychain（ThisDeviceOnly）托管，签名全程不离开进程。
    //
    // macOS 上构建与打包（framework 名 McChainShared）：
    //   ./gradlew :shared:linkReleaseFrameworkIosArm64 :shared:linkReleaseFrameworkIosSimulatorArm64
    //   xcodebuild -create-xcframework \
    //     -framework build/bin/iosArm64/releaseFramework/McChainShared.framework \
    //     -framework build/bin/iosSimulatorArm64/releaseFramework/McChainShared.framework \
    //     -output build/McChainShared.xcframework
    iosArm64 {
        binaries.framework {
            baseName = "McChainShared"
        }
    }
    iosSimulatorArm64 {
        binaries.framework {
            baseName = "McChainShared"
        }
    }

    sourceSets {
        val commonMain by getting
        val commonTest by getting {
            dependencies {
                implementation(kotlin("test"))
            }
        }
        val jvmMain by getting {
            dependencies {
                // 与 app 模块使用同一 BC 变体，避免 jdk15to18 / jdk18on 重复类。
                implementation("org.bouncycastle:bcprov-jdk15to18:1.77")
            }
        }
        val jvmTest by getting {
            dependencies {
                implementation("junit:junit:4.13.2")
                implementation("com.google.code.gson:gson:2.10.1")
            }
        }
    }
}
