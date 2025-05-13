package stages

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/ethclient"
	dstypes "github.com/ledgerwatch/erigon/zk/datastream/types"
	"github.com/ledgerwatch/log/v3"
)

func getMismatchHeight(ctx context.Context, cfg BatchesCfg, dataStreamCatchupCfg DataStreamCatchupCfg) (uint64, bool, error) {
	rpcClientRemote, err := ethclient.Dial(cfg.zkCfg.L2RpcUrl)
	if err != nil {
		return 0, false, fmt.Errorf("ethclient.Dial: %s", err)
	}

	// highest block number
	highestBlockRemote, err := rpcClientRemote.BlockNumber(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("rpcClientRemote.BlockNumber: %s", err)
	}
	highestBlockLocal, err := dataStreamCatchupCfg.dataStreamServer.GetHighestBlockNumber()
	if err != nil {
		return 0, false, fmt.Errorf("downStream.BlockNumber: %s", err)
	}
	highestBlockNumber := highestBlockRemote
	if highestBlockLocal < highestBlockRemote {
		highestBlockNumber = highestBlockLocal
	}

	log.Warn("Starting blockhash mismatch check", "highestBlockRemote", highestBlockRemote, "highestBlockLocal", highestBlockLocal, "working highestBlockNumber", highestBlockNumber)

	var blockRemote *types.Block
	var blockLocal *dstypes.FullL2Block

	for checkBlockNumber := highestBlockNumber; checkBlockNumber > 0; checkBlockNumber-- {
		log.Warn("Checking for block", "blockNumber", checkBlockNumber)
		// get blocks
		blockLocal, blockRemote, err = getBlocks(ctx, rpcClientRemote, checkBlockNumber, dataStreamCatchupCfg)
		if err != nil {
			log.Error(fmt.Sprintf("blockNum: %d, error getBlocks: %s", checkBlockNumber, err))
			return 0, false, err
		}

		isMatch := blockRemote.Hash() == blockLocal.L2Blockhash
		log.Warn("Blockhash check", "blockNumber", checkBlockNumber, "match", isMatch)

		if isMatch {
			return 0, false, nil
		}

		if checkBlockNumber == 1 {
			return checkBlockNumber, true, nil
		}

		prevBlockNumber := checkBlockNumber - 1
		prevBlockLocal, prevBlockRemote, err := getBlocks(ctx, rpcClientRemote, prevBlockNumber, dataStreamCatchupCfg)
		if err != nil {
			log.Error(fmt.Sprintf("prevBlockNum: %d, error getBlocks: %s", prevBlockNumber, err))
			return 0, false, err
		}

		if prevBlockRemote.Hash() == prevBlockLocal.L2Blockhash {
			return checkBlockNumber, true, nil
		}

	}

	return 0, true, nil
}

func getBlocks(ctx context.Context, clientRemote *ethclient.Client, blockNum uint64, dataStreamCatchupCfg DataStreamCatchupCfg) (*dstypes.FullL2Block, *types.Block, error) {
	blockNumBig := new(big.Int).SetUint64(blockNum)
	blockLocal, err := dataStreamCatchupCfg.dataStreamServer.ReadBlock(blockNum)
	if err != nil {
		return nil, nil, fmt.Errorf("downStream.BlockNumber: %s", err)
	}
	blockRemote, err := clientRemote.BlockByNumber(ctx, blockNumBig)
	if err != nil {
		return nil, nil, fmt.Errorf("clientRemote.BlockByNumber: %s", err)
	}
	return blockLocal, blockRemote, nil
}
