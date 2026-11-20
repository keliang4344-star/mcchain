package com.mcchain.miner

/**
 * MCChain 全链参数常量，以创世 genesis.json 与链上模块代码为准。
 * 单一数据源，所有模块共用；修改前必须与创世/链代码核对。
 */
object MCParams {

    // === 链基础 ===
    // 链 ID 由构建注入：debug 固定 mcchain-dev-1；release 默认同为 dev 链，
    // 主网发布时用 -PmcChainId=mcchain-mainnet-1 构建，避免包指向不存在的链。
    val CHAIN_ID: String = BuildConfig.CHAIN_ID
    const val DENOM = "umc"
    const val DISPLAY_DENOM = "MC"
    const val DECIMALS = 6
    const val BECH32_PREFIX = "mc"
    const val TOTAL_SUPPLY_CAP_UMC = 1_000_000_000_000_000L
    const val TOTAL_SUPPLY_MC = 1_000_000_000L
    const val BLOCK_TIME_SECONDS = 4
    const val BLOCKS_PER_YEAR = 6_311_520L  // 取创世 mint.blocks_per_year，勿按 4s 自行推算

    // === 共识 ===
    const val BLOCK_MAX_BYTES = 22_020_096L
    const val TIMEOUT_PROPOSE_MS = 3_000L
    const val TIMEOUT_PREVOTE_MS = 1_000L
    const val TIMEOUT_PRECOMMIT_MS = 1_000L
    const val TIMEOUT_COMMIT_MS = 4_000L

    // === Staking ===
    const val BOND_DENOM = "umc"
    const val UNBONDING_TIME_SECONDS = 1_814_400L // staking.unbonding_time = 1814400s
    const val UNBONDING_DAYS = 21
    const val MAX_VALIDATORS = 100
    const val MAX_ENTRIES = 7

    // === Slashing ===
    const val SIGNED_BLOCKS_WINDOW = 21_600 // slashing.signed_blocks_window
    const val MIN_SIGNED_PERCENT = 50.0
    const val DOWNTIME_JAIL_SECONDS = 600L
    const val SLASH_FRACTION_DOUBLE_SIGN = 0.05
    const val SLASH_FRACTION_DOWNTIME = 0.01

    // === Distribution ===
    const val COMMUNITY_TAX = 0.02

    // === Governance ===
    const val MIN_DEPOSIT_STAKE = 10_000_000L
    const val MAX_DEPOSIT_PERIOD_SECONDS = 172_800L
    const val VOTING_PERIOD_SECONDS = 172_800L
    const val QUORUM_PERCENT = 33.4
    const val PASS_THRESHOLD_PERCENT = 50.0
    const val VETO_THRESHOLD_PERCENT = 33.4

    // === Mint / 通胀 ===
    // 创世 mint 参数当前关闭增发（inflation/min/max/change 均为 0），App 不得展示虚假收益率。
    const val INITIAL_INFLATION = 0.0
    const val INFLATION_RATE_CHANGE = 0.0
    const val INFLATION_MAX = 0.0
    const val INFLATION_MIN = 0.0
    const val GOAL_BONDED = 0.67

    // === PhoneNode ===
    const val PHONENODE_ATTESTATION_REQUIRED = true
    const val PHONENODE_ATTESTATION_VALIDITY_SECONDS = 2_592_000L
    const val PHONENODE_ATTESTATION_VALIDITY_DAYS = 30
    const val PHONENODE_SYBIL_DEVICE_BINDING = true
    const val PHONENODE_OFFLINE_GRACE_BLOCKS = 100L
    const val PHONENODE_OFFLINE_GRACE_SECONDS = PHONENODE_OFFLINE_GRACE_BLOCKS * BLOCK_TIME_SECONDS
    const val PHONENODE_OFFLINE_SLASH_BPS = 500L
    const val PHONENODE_OFFLINE_SLASH_PERCENT = 5.0
    const val PHONENODE_CONTRIB_SLASH_BPS = 1000L
    const val PHONENODE_CONTRIB_SLASH_PERCENT = 10.0
    const val PHONENODE_ATTEST_SLASH_BPS = 2000L
    const val PHONENODE_ATTEST_SLASH_PERCENT = 20.0
    const val PHONENODE_HEARTBEAT_INTERVAL_SECONDS = 20L // 本机轮询间隔，非链上参数（5 块 × 4s）

    // === EdgeAI ===
    const val EDGEAI_DISPUTE_PERIOD_BLOCKS = 100L
    const val EDGEAI_DISPUTE_PERIOD_SECONDS = EDGEAI_DISPUTE_PERIOD_BLOCKS * BLOCK_TIME_SECONDS
    const val EDGEAI_ANTI_CHEAT_THRESHOLD_BPS = 5000L
    const val EDGEAI_ANTI_CHEAT_THRESHOLD_PERCENT = 50.0
    const val EDGEAI_MAX_TASK_REWARD_UMC = 1_000_000_000L

    // === DePIN ===
    const val DEPIN_INITIAL_POOL_UMC = 467_500_000_000_000L // depin.params.initial_pool
    const val DEPIN_REWARD_DENOM = "umc"

    // === Network ===
    const val DEFAULT_RPC_PORT = 26657
    const val DEFAULT_P2P_PORT = 26656
    const val DEFAULT_GRPC_PORT = 9090
    const val DEFAULT_GRPC_WEB_PORT = 9091
    const val DEFAULT_API_PORT = 1317
    const val MAX_INBOUND_PEERS = 40
    const val MAX_OUTBOUND_PEERS = 10
    const val SEND_RATE_BPS = 5_120_000
    const val RECV_RATE_BPS = 5_120_000

    // === Mempool ===
    const val MEMPOOL_SIZE = 5_000
    const val MEMPOOL_MAX_TXS_BYTES = 1_073_741_824L
    const val MEMPOOL_MAX_TX_BYTES = 1_048_576L

    // === Auth ===
    const val MAX_MEMO_CHARACTERS = 256
    const val TX_SIG_LIMIT = 7

    // === IBC ===
    const val MAX_EXPECTED_TIME_PER_BLOCK_NS = 30_000_000_000L

    // === Tokenomics 分配地址（创世 tokenomics.allocations） ===
    const val DEVICE_INCENTIVE_ADDRESS = "mc1lpan9nughhevhdfdu0eywkzllgz39cu8a49jlp"
    const val STAKING_SECURITY_ADDRESS = "mc12fyza25rsnp0jl9yuzses2n3pd5s0ysdxgwrt3"
    const val TEAM_ADDRESS = "mc1psg2qr3d6vhtpcdhh56nd2wxfyted5an95s2c0"
    const val FOUNDATION_ADDRESS = "mc1kqr68y529nexkj80cvkwz0gtnxas0hay3rz0g8"
    const val EARLY_DEV_ADDRESS = "mc1wyzsu27rqk43s4xvce8zcwa5g6svvl5spwqt7k"

    // === 初始分配（5500 / 1500 / 1200 / 1300 / 500 bps，合计 10000） ===
    const val DEVICE_INCENTIVE_BPS = 5500L
    const val DEVICE_INCENTIVE_PERCENT = 55.0
    const val STAKING_SECURITY_BPS = 1500L
    const val STAKING_SECURITY_PERCENT = 15.0
    const val TEAM_ALLOCATION_BPS = 1200L
    const val TEAM_ALLOCATION_PERCENT = 12.0
    const val FOUNDATION_ALLOCATION_BPS = 1300L
    const val FOUNDATION_ALLOCATION_PERCENT = 13.0
    const val EARLY_DEV_ALLOCATION_BPS = 500L
    const val EARLY_DEV_ALLOCATION_PERCENT = 5.0

    // === BIP-44 ===
    const val BIP44_COIN_TYPE = 118 // Cosmos standard
    const val BIP44_PURPOSE = 44
    const val HD_PATH_TEMPLATE = "m/44'/${BIP44_COIN_TYPE}'/0'/0/%d"

    // === 应用级 ===
    const val MIN_STORAGE_MB = 4096L // 4GB minimum for full node
    const val MAX_PEER_CACHE_SIZE = 200
    const val PEER_DIAL_TIMEOUT_SECONDS = 5
    const val PEER_HANDSHAKE_TIMEOUT_SECONDS = 20
    const val SYNC_BATCH_SIZE = 100 // blocks per batch
    const val DB_PRUNE_INTERVAL_BLOCKS = 362_880L // default pruning

    // === 显示用派生参数（SettingsScreen 等引用，全部对齐创世） ===
    const val MIN_SELF_DELEGATE_UMC = 30_000_000_000L // 30,000 MC
    const val DOUBLE_SIGN_SLASH_PERCENT = "5.0"
    const val DOWNTIME_SLASH_PERCENT = "1.0"
    const val DOWNTIME_SLASH_WINDOW_BLOCKS = "21600"
    const val INFLATION_RATE_PERCENT = "0"
    const val TARGET_STAKE_RATIO_PERCENT = "67"
    const val COMMUNITY_TAX_PERCENT = "2"
    const val CONTRIBUTOR_SHARE = "100" // 链上赏金 100% 归贡献节点，不抽成、不销毁
    const val PHONENODE_MIN_STAKE_UMC = 30_000_000_000L // VerifierMinStake = 30,000 MC
}

fun bpsToPercent(bps: Long): Double = bps / 100.0
fun percentToBps(percent: Double): Long = (percent * 100).toLong()
fun umcToMc(umc: Long): Double = umc / 1_000_000.0
fun mcToUmc(mc: Double): Long = (mc * 1_000_000).toLong()

fun String.Companion.hdPath(index: Int): String =
    "m/44'/${MCParams.BIP44_COIN_TYPE}'/0'/0/$index"
