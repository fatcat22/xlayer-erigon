package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon/rpc"
	types "github.com/ledgerwatch/erigon/zk/rpcdaemon"
	"github.com/ledgerwatch/log/v3"
)

func (api *ZkEvmAPIImpl) GetBatchSealTime(ctx context.Context, batchNumber rpc.BlockNumber) (types.ArgUint64, error) {
	lastBatchNo, err := api.BatchNumber(ctx)
	if err != nil {
		return 0, err
	}

	if batchNumber.Int64() >= int64(lastBatchNo) {
		return 0, errors.New(fmt.Sprintf("couldn't get batch number %d's seal time, error: unexpected batch. got %d, last batch should be %d", batchNumber, batchNumber, lastBatchNo))
	}

	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var lastBlockNum = uint64(0)
	lastBlockNum, err = getLastBlockInBatchNumber(tx, uint64(batchNumber.Int64()))
	if err != nil {
		return 0, err
	}

	lastBlock, err := api.GetFullBlockByNumber(ctx, rpc.BlockNumber(lastBlockNum), false)
	if err != nil {
		return 0, err
	}

	return lastBlock.Timestamp, nil
}

func (api *ZkEvmAPIImpl) SendRawTransaction(ctx context.Context, encodedTx hexutility.Bytes) (map[string]interface{}, error) {
	start := time.Now()
	defer func() {
		log.Info("SendRawTransaction completed", "duration", time.Since(start))
	}()

	txnHash, err := api.ethApi.sendRawTransactionSingle(ctx, encodedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send raw transaction: %w for tx %s", err, txnHash.Hex())
	}

	// Poll for the transaction receipt for up to 3 seconds
	timeout := 3 * time.Second
	pollingInterval := 100 * time.Millisecond // Poll every 100ms
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		receipt, err := api.ethApi.GetTransactionReceipt(ctx, txnHash)
		if err != nil {
			log.Debug("Error fetching transaction receipt, will retry", "hash", txnHash, "err", err)
		}

		if receipt != nil {
			return receipt, nil // Receipt found
		}

		select {
		case <-time.After(pollingInterval):
			// Continue to next iteration
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while polling for receipt: %w of tx %s", ctx.Err(), txnHash.Hex())
		}
	}

	return nil, fmt.Errorf("transaction receipt not available after %v for tx %s", timeout, txnHash.Hex())
}
