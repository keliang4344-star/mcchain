package com.mcchain.miner.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.view.View
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.animation.*
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material.icons.outlined.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.core.content.ContextCompat
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.service.MinerService
import com.mcchain.miner.service.NodeService
import com.mcchain.miner.ui.dashboard.DashboardScreen
import com.mcchain.miner.ui.dashboard.DashboardViewModel
import com.mcchain.miner.ui.mining.MiningScreen
import com.mcchain.miner.ui.mining.MiningViewModel
import com.mcchain.miner.ui.node.NodeScreen
import com.mcchain.miner.ui.node.NodeViewModel
import com.mcchain.miner.ui.settings.SettingsScreen
import com.mcchain.miner.ui.settings.SettingsViewModel
import com.mcchain.miner.ui.theme.McMinerTheme
import com.mcchain.miner.ui.wallet.WalletScreen
import com.mcchain.miner.ui.wallet.WalletViewModel
import dagger.hilt.android.AndroidEntryPoint
import timber.log.Timber
import javax.inject.Inject

sealed class Screen(val route: String, val label: String, val icon: ImageVector, val selectedIcon: ImageVector) {
    data object Dashboard : Screen("dashboard", "仪表盘", Icons.Outlined.Dashboard, Icons.Filled.Dashboard)
    data object Wallet : Screen("wallet", "钱包", Icons.Outlined.AccountBalanceWallet, Icons.Filled.AccountBalanceWallet)
    data object Mining : Screen("mining", "挖矿", Icons.Outlined.Memory, Icons.Filled.Memory)
    data object Node : Screen("node", "节点", Icons.Outlined.Hub, Icons.Filled.Hub)
    data object Settings : Screen("settings", "设置", Icons.Outlined.Settings, Icons.Filled.Settings)
}

@AndroidEntryPoint
class MainActivity : ComponentActivity() {

    @Inject lateinit var securePrefs: SecurePrefs

    // Compose 的 viewModel() 使用 NavBackStackEntry 工厂，无法创建 @HiltViewModel；
    // 因此在 Activity（Hilt 注入默认工厂）作用域内创建，再传入各页面。
    private val dashboardViewModel: DashboardViewModel by viewModels()
    private val walletViewModel: WalletViewModel by viewModels()
    private val miningViewModel: MiningViewModel by viewModels()
    private val nodeViewModel: NodeViewModel by viewModels()
    private val settingsViewModel: SettingsViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            window.setDecorFitsSystemWindows(false)
        } else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_LAYOUT_STABLE or View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
        }

        reconcileServices()

        setContent {
            McMinerTheme {
                MainApp(
                    dashboardViewModel = dashboardViewModel,
                    walletViewModel = walletViewModel,
                    miningViewModel = miningViewModel,
                    nodeViewModel = nodeViewModel,
                    settingsViewModel = settingsViewModel
                )
            }
        }
    }

    /** 让开关（偏好）与服务实际状态保持一致：开关为开但服务未运行时就补启动。 */
    private fun reconcileServices() {
        runCatching {
            if (securePrefs.nodeSyncEnabled && !NodeService.isRunning) NodeService.start(this)
            if (securePrefs.miningEnabled && !MinerService.isRunning) MinerService.start(this)
        }.onFailure { Timber.w(it, "Failed to reconcile foreground services") }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainApp(
    dashboardViewModel: DashboardViewModel,
    walletViewModel: WalletViewModel,
    miningViewModel: MiningViewModel,
    nodeViewModel: NodeViewModel,
    settingsViewModel: SettingsViewModel
) {
    val navController = rememberNavController()
    val screens = listOf(
        Screen.Dashboard,
        Screen.Wallet,
        Screen.Mining,
        Screen.Node,
        Screen.Settings
    )

    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = navBackStackEntry?.destination

    // Android 13+ 前台服务通知需要运行时授权，否则常驻通知不可见。
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
        val context = LocalContext.current
        val notificationPermission = rememberLauncherForActivityResult(
            ActivityResultContracts.RequestPermission()
        ) { }
        LaunchedEffect(Unit) {
            val granted = ContextCompat.checkSelfPermission(
                context, Manifest.permission.POST_NOTIFICATIONS
            ) == PackageManager.PERMISSION_GRANTED
            if (!granted) notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }

    Scaffold(
        bottomBar = {
            NavigationBar(
                containerColor = MaterialTheme.colorScheme.surface,
                contentColor = MaterialTheme.colorScheme.primary
            ) {
                screens.forEach { screen ->
                    val selected = currentDestination?.hierarchy?.any { it.route == screen.route } == true
                    NavigationBarItem(
                        icon = {
                            Icon(
                                imageVector = if (selected) screen.selectedIcon else screen.icon,
                                contentDescription = screen.label
                            )
                        },
                        label = { Text(screen.label) },
                        selected = selected,
                        onClick = {
                            navController.navigate(screen.route) {
                                popUpTo(navController.graph.findStartDestination().id) {
                                    saveState = true
                                }
                                launchSingleTop = true
                                restoreState = true
                            }
                        }
                    )
                }
            }
        }
    ) { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = Screen.Dashboard.route,
            modifier = Modifier.padding(innerPadding)
        ) {
            composable(Screen.Dashboard.route) {
                DashboardScreen(
                    viewModel = dashboardViewModel,
                    onOpenWallet = {
                        navController.navigate(Screen.Wallet.route) { launchSingleTop = true }
                    },
                    onOpenNode = {
                        navController.navigate(Screen.Node.route) { launchSingleTop = true }
                    },
                    onOpenMining = {
                        navController.navigate(Screen.Mining.route) { launchSingleTop = true }
                    }
                )
            }
            composable(Screen.Wallet.route) { WalletScreen(viewModel = walletViewModel) }
            composable(Screen.Mining.route) {
                MiningScreen(
                    viewModel = miningViewModel,
                    onOpenWallet = {
                        navController.navigate(Screen.Wallet.route) { launchSingleTop = true }
                    },
                    onOpenNode = {
                        navController.navigate(Screen.Node.route) { launchSingleTop = true }
                    }
                )
            }
            composable(Screen.Node.route) {
                NodeScreen(
                    vm = nodeViewModel,
                    onOpenSettings = {
                        navController.navigate(Screen.Settings.route) { launchSingleTop = true }
                    },
                    onOpenWallet = {
                        navController.navigate(Screen.Wallet.route) { launchSingleTop = true }
                    }
                )
            }
            composable(Screen.Settings.route) { SettingsScreen(viewModel = settingsViewModel) }
        }
    }
}
