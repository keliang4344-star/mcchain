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
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.service.MinerService
import com.mcchain.miner.service.NodeService
import com.mcchain.miner.ui.dashboard.DashboardScreen
import com.mcchain.miner.ui.dashboard.DashboardViewModel
import com.mcchain.miner.ui.ecosystem.EdgeAiScreen
import com.mcchain.miner.ui.ecosystem.EcosystemViewModel
import com.mcchain.miner.ui.ecosystem.ReferralScreen
import com.mcchain.miner.ui.ecosystem.TokenomicsScreen
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

/** 非底部导航的二级页面路由。 */
private object Route {
    const val REFERRAL = "referral"
    const val EDGE_AI = "edge_ai"
    const val TOKENOMICS = "tokenomics"
}

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
    private val ecosystemViewModel: EcosystemViewModel by viewModels()

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
                    settingsViewModel = settingsViewModel,
                    ecosystemViewModel = ecosystemViewModel
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
    settingsViewModel: SettingsViewModel,
    ecosystemViewModel: EcosystemViewModel
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

    // 二级页面（推荐/EdgeAI/代币经济）不显示底部导航，避免误触切页。
    val showBottomBar = screens.any { it.route == currentDestination?.route }

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
            if (showBottomBar) NavigationBar(
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
            composable(Screen.Settings.route) {
                SettingsScreen(
                    viewModel = settingsViewModel,
                    onOpenReferral = { navController.navigate(Route.REFERRAL) { launchSingleTop = true } },
                    onOpenEdgeAi = { navController.navigate(Route.EDGE_AI) { launchSingleTop = true } },
                    onOpenTokenomics = { navController.navigate(Route.TOKENOMICS) { launchSingleTop = true } },
                )
            }
            composable(Route.REFERRAL) {
                ReferralScreen(viewModel = ecosystemViewModel) { navController.popBackStack() }
            }
            composable(Route.EDGE_AI) {
                EdgeAiScreen(viewModel = ecosystemViewModel) { navController.popBackStack() }
            }
            composable(Route.TOKENOMICS) {
                TokenomicsScreen(viewModel = ecosystemViewModel) { navController.popBackStack() }
            }
        }
    }

    // 首次启动引导：RPC 端点决定 App 能否读取链上数据，先让用户确认或填写。
    val prefs = settingsViewModel.securePrefs
    var showRpcGuide by remember {
        mutableStateOf(
            runCatching { prefs.isFirstLaunch && prefs.rpcEndpoint.isBlank() }.getOrDefault(false)
        )
    }
    if (showRpcGuide) {
        FirstLaunchRpcGuide(
            viewModel = settingsViewModel,
            onDone = {
                runCatching { prefs.isFirstLaunch = false }
                showRpcGuide = false
            },
        )
    }
}

/**
 * 首次启动的 RPC 引导。
 * 只做手动填写与连通性测试：官方公共 RPC 尚未发布，因此暂不提供固定二维码。
 */
@Composable
private fun FirstLaunchRpcGuide(viewModel: SettingsViewModel, onDone: () -> Unit) {
    val savedEndpoint by viewModel.rpcEndpoint.collectAsState()
    val testResult by viewModel.rpcTestResult.collectAsState()
    var endpoint by remember { mutableStateOf(savedEndpoint) }
    var error by remember { mutableStateOf<String?>(null) }
    val clipboard = LocalClipboardManager.current

    AlertDialog(
        onDismissRequest = onDone,
        title = { Text("先连接 MC 网络") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    text = "App 只做链上查询与贡献计量，不运行全节点、不参与 PoW 出块。" +
                        "请填写可访问的 RPC 节点地址（默认端口 ${MCParams.DEFAULT_RPC_PORT}）。",
                    style = MaterialTheme.typography.bodySmall,
                )
                OutlinedTextField(
                    value = endpoint,
                    onValueChange = {
                        endpoint = it
                        error = null
                    },
                    label = { Text("RPC 端点") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    TextButton(onClick = {
                        val text = runCatching { clipboard.getText()?.text }
                            .getOrNull()?.trim().orEmpty()
                        if (text.startsWith("http://") || text.startsWith("https://")) {
                            endpoint = text
                            error = null
                        } else {
                            error = "剪贴板中没有有效的 http(s) 地址"
                        }
                    }) { Text("粘贴") }
                    TextButton(onClick = { viewModel.testRpcConnection(endpoint) }) {
                        Text("测试连接")
                    }
                }
                testResult?.let {
                    Text(
                        text = it,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.75f),
                    )
                }
                error?.let {
                    Text(
                        text = it,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                }
                Text(
                    text = "官方公共 RPC 尚未发布，当前请使用自建或受信任节点；公共节点上线后会提供二维码导入。",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                )
            }
        },
        confirmButton = {
            TextButton(onClick = {
                val value = endpoint.trim().trimEnd('/')
                when {
                    value.isBlank() -> error = "请输入 RPC 端点"
                    !value.startsWith("http://") && !value.startsWith("https://") ->
                        error = "端点必须以 http:// 或 https:// 开头"
                    else -> {
                        viewModel.updateRpcEndpoint(value)
                        onDone()
                    }
                }
            }) { Text("保存并开始") }
        },
        dismissButton = {
            TextButton(onClick = onDone) { Text("稍后设置") }
        },
    )
}
