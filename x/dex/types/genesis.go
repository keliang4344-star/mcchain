package types

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Pools:       []Pool{},
		NextPoolId:  1,
		Params:      DefaultParams(),
	}
}

func (gs GenesisState) Validate() error {
	if gs.Params.DefaultFeeRateBps > 10000 {
		return ErrInvalidFeeRate
	}
	if gs.Params.MaxPools == 0 {
		return ErrInvalidMaxPools
	}
	for _, pool := range gs.Pools {
		if pool.DenomA == "" || pool.DenomB == "" {
			return ErrInvalidDenom
		}
		if pool.DenomA == pool.DenomB {
			return ErrDuplicateDenom
		}
	}
	return nil
}
