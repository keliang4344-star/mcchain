import * as crypto from "@cosmjs/crypto";
import * as amino from "@cosmjs/amino";
import * as protoSigning from "@cosmjs/proto-signing";
import * as tendermintRpc from "@cosmjs/tendermint-rpc";
import * as stargate from "@cosmjs/stargate";

// 统一挂到 window.cosmjs，保持 miner.html 现有调用方式不变
window.cosmjs = { crypto, amino, protoSigning, tendermintRpc, stargate };
