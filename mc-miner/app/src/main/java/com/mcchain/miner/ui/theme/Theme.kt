package com.mcchain.miner.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

private val DarkColorScheme = darkColorScheme(
    // 深色背景上使用浅蓝主色，保证图标与按钮文字达到可读对比度（原 McAccent 在深色背景上对比不足）。
    primary = Color(0xFF8AB4F8),
    secondary = McHighlight,
    tertiary = McGold,
    background = McBackgroundDark,
    surface = McSurfaceDark,
    onPrimary = Color(0xFF0B1B2B),
    onSecondary = Color.White,
    onTertiary = Color.Black,
    onBackground = McOnBackgroundDark,
    onSurface = McOnSurfaceDark,
    surfaceVariant = McCardDark,
    onSurfaceVariant = Color(0xFFC9D1D9),
    outline = McDividerDark
)

private val LightColorScheme = lightColorScheme(
    primary = McAccent,
    secondary = McHighlight,
    tertiary = McGold,
    background = McBackgroundLight,
    surface = McSurfaceLight,
    onPrimary = Color.White,
    onSecondary = Color.White,
    onTertiary = Color.Black,
    onBackground = McOnSurfaceLight,
    onSurface = McOnSurfaceLight,
    surfaceVariant = McGold.copy(alpha = 0.12f),
    outline = McDividerLight
)

@Composable
fun McMinerTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme = if (darkTheme) DarkColorScheme else LightColorScheme

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as? Activity)?.window ?: return@SideEffect
            window.statusBarColor = colorScheme.background.toArgb()
            WindowCompat.getInsetsController(window, view).isAppearanceLightStatusBars = !darkTheme
        }
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = Typography(),
        content = content
    )
}
