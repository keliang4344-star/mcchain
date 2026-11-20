# OkHttp
-dontwarn okhttp3.**
-dontwarn okio.**
-keep class okhttp3.** { *; }
-keep interface okhttp3.** { *; }

# Gson
-keepattributes Signature
-keepattributes *Annotation*
-keep class com.google.gson.** { *; }
-keep class com.mcchain.miner.domain.wallet.TxBuilder$** { *; }
-keep class com.mcchain.miner.network.RpcClient$** { *; }

# Room
-keep class * extends androidx.room.RoomDatabase
-keep @androidx.room.Entity class *
-dontwarn androidx.room.paging.**

# Bitcoinj
-keep class org.bitcoinj.** { *; }
-dontwarn org.bitcoinj.**

# slf4j（bitcoinj 传递依赖，运行期用 no-op 绑定即可，R8 不需要 StaticLoggerBinder）
-dontwarn org.slf4j.**

# BouncyCastle
-keep class org.bouncycastle.** { *; }
-dontwarn org.bouncycastle.jce.provider.**

# Hilt
-keep class dagger.hilt.** { *; }
-keep class javax.inject.** { *; }
-keep class * extends dagger.hilt.android.internal.managers.ViewComponentManager$FragmentContextWrapper { *; }

# Hilt generated classes
-keep class com.mcchain.miner.Hilt_* { *; }
-keep class com.mcchain.miner.ui.Hilt_* { *; }
-keep class com.mcchain.miner.service.Hilt_* { *; }

# Keep crash log
-keep class java.io.** { *; }

# Coroutines
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}

# MC Miner models (needed for Gson serialization)
-keep class com.mcchain.miner.data.model.** { *; }

# Timber
-dontwarn org.jetbrains.annotations.**

# Compose animation-core
# R8 会抹掉 KeyframesSpecBaseConfig 上被继承的 at()/atFraction()，导致 release 包在不定
# 进度条等 keyframes 动画路径上抛 NoSuchMethodError（debug 包不受影响）。
-keep class androidx.compose.animation.core.** { *; }

# Generic
-keepattributes InnerClasses
-keepattributes EnclosingMethod
