package com.mcchain.miner.ui.settings

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.mcchain.miner.ui.theme.McMinerTheme
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test

/**
 * 设置页无状态组件的最小 UI 测试，作为 Compose 测试基础设施的回归样例。
 * 使用 createComposeRule，不启动 Hilt/Activity，避免依赖链上环境。
 */
class SettingsComponentsTest {

    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun noteSettingItemRendersWholeText() {
        val text = "节点同步与贡献挖矿属于 Android 前台服务：系统要求始终保持一条最小常驻通知。"
        composeRule.setContent {
            McMinerTheme { NoteSettingItem(text) }
        }
        composeRule.onNodeWithText(text).assertIsDisplayed()
    }

    @Test
    fun switchSettingItemReportsToggle() {
        var toggled: Boolean? = null
        composeRule.setContent {
            McMinerTheme {
                SwitchSettingItem(
                    icon = Icons.Filled.Notifications,
                    title = "常驻通知（测试）",
                    description = "关闭后仅隐藏详细状态",
                    checked = false,
                    onCheckedChange = { toggled = it },
                )
            }
        }
        composeRule.onNodeWithText("常驻通知（测试）").assertIsDisplayed()
        composeRule.onNode(isToggleable()).performClick()
        composeRule.runOnIdle { assertEquals(true, toggled) }
    }
}
