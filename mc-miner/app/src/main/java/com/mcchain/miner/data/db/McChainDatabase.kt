package com.mcchain.miner.data.db

import androidx.room.Database
import androidx.room.RoomDatabase
import com.mcchain.miner.data.model.*

@Database(
    entities = [
        BlockInfo::class,
        TxRecord::class,
        PeerNode::class,
        WalletAccount::class,
        PhoneNodeRecord::class,
        Contribution::class,
        EdgeAiTask::class,
        NodeStatusSnapshot::class
    ],
    version = 1,
    exportSchema = false
)
abstract class McChainDatabase : RoomDatabase() {
    abstract fun blockDao(): BlockDao
    abstract fun txDao(): TxDao
    abstract fun peerDao(): PeerDao
    abstract fun accountDao(): AccountDao
    abstract fun phoneNodeDao(): PhoneNodeDao
    abstract fun contributionDao(): ContributionDao
    abstract fun edgeAiTaskDao(): EdgeAiTaskDao
    abstract fun nodeStatusDao(): NodeStatusDao
}
