pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    // KMP 的 Kotlin/Native 目标会自动为 shared 注入一个 ivy 仓库（konan 依赖），
    // FAIL_ON_PROJECT_REPOS 会直接拒绝它；这里放宽为项目仓库优先，
    // 实际依赖解析仍以 settings 中的 mavenCentral/google 为主。
    repositoriesMode.set(RepositoriesMode.PREFER_PROJECT)
    repositories {
        google()
        mavenCentral()
        maven { url = uri("https://jitpack.io") }
    }
}

rootProject.name = "mc-miner-native"
include(":app")
include(":shared")
