package com.mcchain.miner.data.db

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
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
        NodeStatusSnapshot::class,
        AddressBookEntry::class
    ],
    version = 2,
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
    abstract fun addressBookDao(): AddressBookDao

    companion object {
        /**
         * v2 新增地址簿表。
         * 不能依赖 fallbackToDestructiveMigration：那会在升级时清空钱包账户与交易记录。
         */
        val MIGRATION_1_2 = object : Migration(1, 2) {
            override fun migrate(db: SupportSQLiteDatabase) {
                db.execSQL(
                    "CREATE TABLE IF NOT EXISTS `address_book` (" +
                        "`id` INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, " +
                        "`address` TEXT NOT NULL, " +
                        "`label` TEXT NOT NULL, " +
                        "`createdAt` INTEGER NOT NULL)"
                )
                db.execSQL(
                    "CREATE UNIQUE INDEX IF NOT EXISTS `index_address_book_address` " +
                        "ON `address_book` (`address`)"
                )
            }
        }
    }
}
