// MC Chain Message Types
declare namespace MC {
    // EdgeAI
    interface MsgCreateTask {
        typeUrl: '/mcchain.edgeai.MsgCreateTask';
        value: {
            creator: string;
            taskType: string;
            description: string;
            dataUri: string;
            reward: string;
            assignee: string;
            timeoutBlocks: number;
        };
    }

    interface MsgSubmitResult {
        typeUrl: '/mcchain.edgeai.MsgSubmitResult';
        value: {
            creator: string;
            taskId: string;
            resultHash: string;
            attestationNonce: string;
        };
    }

    interface MsgOpenDispute {
        typeUrl: '/mcchain.edgeai.MsgOpenDispute';
        value: {
            creator: string;
            taskId: string;
            reason: string;
        };
    }

    interface MsgResolveDispute {
        typeUrl: '/mcchain.edgeai.MsgResolveDispute';
        value: {
            creator: string;
            taskId: string;
            resolution: string; // "honest" | "cheat"
        };
    }

    // DePIN
    interface MsgRegisterDevice {
        typeUrl: '/mcchain.depin.MsgRegisterDevice';
        value: {
            creator: string;
            address: string;
            model: string;
            os: string;
        };
    }

    interface MsgSubmitContribution {
        typeUrl: '/mcchain.depin.MsgSubmitContribution';
        value: {
            creator: string;
            taskType: string;
            proofData: string;
            deviceId: string;
        };
    }

    interface MsgClaimReward {
        typeUrl: '/mcchain.depin.MsgClaimReward';
        value: {
            creator: string;
        };
    }

    // Phonenode
    interface MsgRegisterNode {
        typeUrl: '/mcchain.phonenode.MsgRegisterNode';
        value: {
            creator: string;
            address: string;
            model: string;
            os: string;
            role: string;
        };
    }

    interface MsgAttestDevice {
        typeUrl: '/mcchain.phonenode.MsgAttestDevice';
        value: {
            creator: string;
            deviceInfo: string;
            proof: string;
        };
    }

    // DEX
    interface MsgCreatePool {
        typeUrl: '/mcchain.dex.MsgCreatePool';
        value: {
            creator: string;
            denomA: string;
            denomB: string;
            amountA: string;
            amountB: string;
            feeRateBps: number;
            poolId: number;
        };
    }

    interface MsgSwapExactIn {
        typeUrl: '/mcchain.dex.MsgSwapExactIn';
        value: {
            creator: string;
            poolId: number;
            denomIn: string;
            amountIn: string;
            denomOut: string;
            minAmountOut: string;
        };
    }

    interface MsgAddLiquidity {
        typeUrl: '/mcchain.dex.MsgAddLiquidity';
        value: {
            creator: string;
            poolId: number;
            amountAMax: string;
            amountBMax: string;
            minLpOut: string;
        };
    }

    interface MsgRemoveLiquidity {
        typeUrl: '/mcchain.dex.MsgRemoveLiquidity';
        value: {
            creator: string;
            poolId: number;
            lpAmount: string;
            minAOut: string;
            minBOout: string;
        };
    }

    // Referral
    interface MsgCreateReferral {
        typeUrl: '/mcchain.referral.MsgCreateReferral';
        value: {
            inviter: string;
            invitee: string;
            inviteCode: string;
        };
    }

    interface MsgClaimReferralReward {
        typeUrl: '/mcchain.referral.MsgClaimReferralReward';
        value: {
            claimer: string;
        };
    }

    // Query responses
    interface EdgeAITask {
        id: string;
        creator: string;
        taskType: string;
        reward: string;
        status: string;
        assignee: string;
        createdAt: number;
    }

    interface ReferralInfo {
        id: number;
        inviter: string;
        invitee: string;
        status: string;
        pendingReward: string;
        claimedReward: string;
    }
}
