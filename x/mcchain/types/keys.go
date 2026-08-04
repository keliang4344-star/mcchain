package types

const (
	// ModuleName defines the module name
	ModuleName = "mcchain"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_mcchain"
)

var (
	// HeartbeatKey stores the latest block height for chain heartbeat
	HeartbeatKey = KeyPrefix("Heartbeat")

	// GovernanceHandoverConfigKey 渐进治理移交配置（JSON）在模块 KVStore 中的键。
	GovernanceHandoverConfigKey = []byte("GovHandoverCfg:")
)

// ChainInfo carries the system anchor information (whitepaper line 590)
type ChainInfo struct {
	ChainName     string `json:"chain_name"`
	ChainVersion  string `json:"chain_version"`
	GenesisTime   int64  `json:"genesis_time"`
	BlockHeight   int64  `json:"block_height"`
	LastHeartbeat int64  `json:"last_heartbeat"`
}

func KeyPrefix(p string) []byte {
	return []byte(p)
}
