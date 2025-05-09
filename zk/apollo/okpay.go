package apollo

import libcommon "github.com/ledgerwatch/erigon-lib/common"

// Note: Both pool and sequencer namespaces are allowed to set and use dynamic OkPay configs

// GetOkPayBlockTxsLimit returns the OkPay block tx limit
func GetOkPayBlockTxsLimit(localOkPayBlockTxsLimit uint64) uint64 {
	if IsApolloConfigSequencerEnabled() || IsApolloConfigPoolEnabled() {
		UnsafeGetApolloConfig().RLock()
		defer UnsafeGetApolloConfig().RUnlock()
		return UnsafeGetApolloConfig().EthCfg.Zk.XLayer.SequencerOkPayBlockTxsLimit
	}
	return localOkPayBlockTxsLimit
}

// CheckOkPayAddress checks if the address is in the OkPay accounts list
func CheckOkPayAddress(localOkPayAccountsList libcommon.OrderedList[libcommon.Address], addr libcommon.Address) bool {
	if IsApolloConfigSequencerEnabled() || IsApolloConfigPoolEnabled() {
		UnsafeGetApolloConfig().RLock()
		defer UnsafeGetApolloConfig().RUnlock()
		return UnsafeGetApolloConfig().EthCfg.Zk.XLayer.OkPaySenderAccountsList.Contains(addr)
	}
	return localOkPayAccountsList.Contains(addr)
}

// CheckOkPayAddress checks if the address is in the OkPay accounts list
func (cfg *ApolloConfig) CheckOkPayAddress(localOkPayAccountsList libcommon.OrderedList[libcommon.Address], addr libcommon.Address) bool {
	cfg.RLock()
	defer cfg.RUnlock()

	if cfg.isPoolEnabled() {
		return cfg.EthCfg.Zk.XLayer.OkPaySenderAccountsList.Contains(addr)
	}
	return localOkPayAccountsList.Contains(addr)
}

// GetOkPayYieldGasPercentageLimit returns the OkPay yield gas percentage limit
func (cfg *ApolloConfig) GetOkPayYieldGasPercentageLimit(localOkPayYieldGasPercentageLimit float64) float64 {
	cfg.RLock()
	defer cfg.RUnlock()

	if cfg.isPoolEnabled() || cfg.isSeqEnabled() {
		return cfg.EthCfg.DeprecatedTxPool.OkPayYieldGasPercentageLimit
	}
	return localOkPayYieldGasPercentageLimit
}
