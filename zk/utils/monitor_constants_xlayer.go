package utils

type TransactionStatus string
type Client string

type ProcessStep struct {
	ID  int
	Key string
}

var (
	// RPC
	StepRPCReceiveTx    = ProcessStep{15010, "xlayer_rpc_receive_tx"}
	StepRPCForwardTx    = ProcessStep{15012, "xlayer_rpc_forward_tx"}
	StepRPCReceiveBlock = ProcessStep{15060, "xlayer_rpc_receive_block"}
	StepRPCFinishBlock  = ProcessStep{15062, "xlayer_rpc_finish_block"}

	// Pool Manager
	StepPoolMgrReceiveTx = ProcessStep{15020, "xlayer_plmgr_receive_tx"}
	StepPoolMgrForwardTx = ProcessStep{15022, "xlayer_plmgr_forward_tx"}

	// Sequencer
	StepSeqBeginBlock     = ProcessStep{15030, "xlayer_seq_begin_block"}
	StepSeqReceiveTx      = ProcessStep{15032, "xlayer_seq_receive_tx"}
	StepSeqPackageTx      = ProcessStep{15034, "xlayer_seq_package_tx"}
	StepSeqEndBlock       = ProcessStep{15036, "xlayer_seq_end_block"}
	StepSeqVerifyTxBegin  = ProcessStep{15038, "xlayer_seq_verify_tx_begin"}
	StepSeqVerifyTxResult = ProcessStep{15040, "xlayer_seq_verify_tx_result"}
	StepSeqDsSent         = ProcessStep{15042, "xlayer_seq_ds_sent"}

	// Data Stream
	StepDsReceiveBlock = ProcessStep{15050, "xlayer_ds_receive_block"}
)

const (
	Chain = "xlayer"

	ServiceNameRPC         = "okx-defi-xlayer-rpcpay-pro"
	ServiceNamePoolManager = "okx-defi-xlayer-plmgr-pro"
	ServiceNameSequencer   = "okx-defi-xlayer-egseqz-pro"

	Business = "xlayer"
	ChainID  = 196

	StatusPending        TransactionStatus = "pending"
	StatusConfirm        TransactionStatus = "confirm"
	StatusDropAndReplace TransactionStatus = "drop&replace"
	StatusDrop           TransactionStatus = "drop"
	StatusFilter         TransactionStatus = "filter"

	ClientWeb     Client = "web"
	ClientIOS     Client = "ios"
	ClientAndroid Client = "android"

	ProcessWord
)
