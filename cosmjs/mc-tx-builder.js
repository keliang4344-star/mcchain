// MC Chain Transaction Builder
// 依赖 cosmjs (StargateClient, SigningStargateClient)
// 用于所有 MC 自定义模块的交易签名和广播

const MC = {
    // 将数字 MC 金额转换为 umc 字符串
    toUmc: (mc) => (BigInt(mc) * 1000000n).toString(),
    
    // 将 umc 字符串转换为数字 MC
    fromUmc: (umc) => Number(BigInt(umc) / 1000000n),

    // EdgeAI
    edgeai: {
        createTask: (creator, taskType, description, dataUri, rewardUmc, assignee) => ({
            typeUrl: '/mcchain.edgeai.MsgCreateTask',
            value: { creator, taskType, description, dataUri, reward: rewardUmc, assignee, timeoutBlocks: '10000' }
        }),
        submitResult: (creator, taskId, resultHash, attestationNonce) => ({
            typeUrl: '/mcchain.edgeai.MsgSubmitResult',
            value: { creator, taskId, resultHash, attestationNonce }
        }),
        openDispute: (creator, taskId, reason) => ({
            typeUrl: '/mcchain.edgeai.MsgOpenDispute',
            value: { creator, taskId, reason }
        }),
        resolveDispute: (arbitrator, taskId, resolution) => ({
            typeUrl: '/mcchain.edgeai.MsgResolveDispute',
            value: { creator: arbitrator, taskId, resolution }
        })
    },

    // DePIN
    depin: {
        registerDevice: (creator, model, os) => ({
            typeUrl: '/mcchain.depin.MsgRegisterDevice',
            value: { creator, address: creator, model, os }
        }),
        submitContribution: (creator, taskType, proofData, deviceId) => ({
            typeUrl: '/mcchain.depin.MsgSubmitContribution',
            value: { creator, taskType, proofData, deviceId }
        }),
        claimReward: (creator) => ({
            typeUrl: '/mcchain.depin.MsgClaimReward',
            value: { creator }
        })
    },

    // Phonenode
    phonenode: {
        registerNode: (creator, model, os, role) => ({
            typeUrl: '/mcchain.phonenode.MsgRegisterNode',
            value: { creator, address: creator, model, os, role }
        }),
        attestDevice: (creator, deviceInfo, proof) => ({
            typeUrl: '/mcchain.phonenode.MsgAttestDevice',
            value: { creator, deviceInfo, proof }
        })
    },

    // DEX
    dex: {
        createPool: (creator, denomA, denomB, amountA, amountB, feeRateBps, poolId) => ({
            typeUrl: '/mcchain.dex.MsgCreatePool',
            value: { creator, denomA, denomB, amountA, amountB, feeRateBps, poolId }
        }),
        swapExactIn: (creator, poolId, denomIn, amountIn, denomOut, minAmountOut) => ({
            typeUrl: '/mcchain.dex.MsgSwapExactIn',
            value: { creator, poolId, denomIn, amountIn, denomOut, minAmountOut }
        }),
        addLiquidity: (creator, poolId, amountAMax, amountBMax, minLpOut) => ({
            typeUrl: '/mcchain.dex.MsgAddLiquidity',
            value: { creator, poolId, amountAMax, amountBMax, minLpOut }
        }),
        removeLiquidity: (creator, poolId, lpAmount, minAOut, minBOut) => ({
            typeUrl: '/mcchain.dex.MsgRemoveLiquidity',
            value: { creator, poolId, lpAmount, minAOut, minBOut }
        })
    },

    // Referral
    referral: {
        createReferral: (inviter, invitee, inviteCode) => ({
            typeUrl: '/mcchain.referral.MsgCreateReferral',
            value: { inviter, invitee, inviteCode }
        }),
        claimReward: (claimer) => ({
            typeUrl: '/mcchain.referral.MsgClaimReferralReward',
            value: { claimer }
        })
    }
};

// 导出（兼容浏览器 script 标签和 module 两种加载方式）
if (typeof module !== 'undefined' && module.exports) {
    module.exports = MC;
} else if (typeof window !== 'undefined') {
    window.MC = MC;
}
