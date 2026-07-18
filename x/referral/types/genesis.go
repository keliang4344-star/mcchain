package types

// GenesisState defines the referral module's genesis state.
// Mirrors proto/mcchain/referral/genesis.proto.
type GenesisState struct {
	Params Params `json:"params"`
}

func (gs *GenesisState) Reset()        { *gs = GenesisState{} }
func (gs *GenesisState) String() string { return "GenesisState" }
func (gs *GenesisState) ProtoMessage()  {}

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
