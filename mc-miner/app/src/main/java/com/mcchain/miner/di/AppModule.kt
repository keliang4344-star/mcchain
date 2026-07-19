package com.mcchain.miner.di

import android.content.Context
import androidx.room.Room
import com.google.gson.Gson
import com.mcchain.miner.data.db.McChainDatabase
import com.mcchain.miner.data.pref.SecurePrefs
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object AppModule {

    @Provides
    @Singleton
    fun provideGson(): Gson = Gson()

    @Provides
    @Singleton
    fun provideDatabase(@ApplicationContext context: Context): McChainDatabase {
        return Room.databaseBuilder(
            context,
            McChainDatabase::class.java,
            "mcchain_db"
        )
            .fallbackToDestructiveMigration()
            .build()
    }

    @Provides
    @Singleton
    fun provideBlockDao(db: McChainDatabase) = db.blockDao()

    @Provides
    @Singleton
    fun provideTxDao(db: McChainDatabase) = db.txDao()

    @Provides
    @Singleton
    fun providePeerDao(db: McChainDatabase) = db.peerDao()

    @Provides
    @Singleton
    fun provideAccountDao(db: McChainDatabase) = db.accountDao()

    @Provides
    @Singleton
    fun providePhoneNodeDao(db: McChainDatabase) = db.phoneNodeDao()

    @Provides
    @Singleton
    fun provideContributionDao(db: McChainDatabase) = db.contributionDao()

    @Provides
    @Singleton
    fun provideEdgeAiTaskDao(db: McChainDatabase) = db.edgeAiTaskDao()

    @Provides
    @Singleton
    fun provideNodeStatusDao(db: McChainDatabase) = db.nodeStatusDao()

    @Provides
    @Singleton
    fun provideSecurePrefs(@ApplicationContext context: Context): SecurePrefs {
        return SecurePrefs(context)
    }
}
