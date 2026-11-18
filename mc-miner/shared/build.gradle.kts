plugins {
    kotlin("multiplatform")
}

kotlin {
    jvmToolchain(17)

    // Phase 0 只启用 JVM 目标（Android 直接消费）。
    // iOS 目标（iosArm64/iosX64/iosSimulatorArm64）在 Phase 1 接入，届时只需在此追加。
    jvm {
        compilations.all {
            kotlinOptions.jvmTarget = "17"
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
                // 仅 JVM/Android 侧需要 secp256k1 签名；iOS actual 在 Phase 1 用 secp256k1-kmp 实现。
                implementation("org.bouncycastle:bcprov-jdk18on:1.77")
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
