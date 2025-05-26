package txpool

import (
	"context"
	"math"
	"time"

	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/log/v3"
)

type LimboSubPoolProcessor struct {
	zkCfg       *ethconfig.Zk
	chainConfig *chain.Config
	db          kv.RwDB
	txPool      *TxPool
	quit        <-chan struct{}
}

func NewLimboSubPoolProcessor(ctx context.Context, zkCfg *ethconfig.Zk, chainConfig *chain.Config, db kv.RwDB, txPool *TxPool) *LimboSubPoolProcessor {
	return &LimboSubPoolProcessor{
		zkCfg:       zkCfg,
		chainConfig: chainConfig,
		db:          db,
		txPool:      txPool,
		quit:        ctx.Done(),
	}
}

func (_this *LimboSubPoolProcessor) StartWork() {
	go func() {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
	LOOP:
		for {
			select {
			case <-_this.quit:
				break LOOP
			case <-tick.C:
				_this.run()
			}
		}
	}()
}

func (_this *LimboSubPoolProcessor) run() {
	log.Info("[Limbo pool processor] Starting")
	defer log.Info("[Limbo pool processor] End")

	ctx := context.Background()
	limboBlocksDetails := _this.txPool.GetUncheckedLimboBlocksDetailsClonedWeak()

	size := len(limboBlocksDetails)
	if size == 0 {
		return
	}

	totalTransactions := 0
	for _, limboBlock := range limboBlocksDetails {
		for _, limboTx := range limboBlock.Transactions {
			if !limboTx.hasRoot() {
				return
			}
			totalTransactions++
		}
	}

	tx, err := _this.db.BeginRo(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback()

	// we just need some counter variable with large used values in order verify not to complain
	batchCounters := vm.NewBatchCounterCollector(256, 1, _this.zkCfg.VirtualCountersSmtReduction, true, nil)
	unlimitedCounters := batchCounters.NewCounters().UsedAsMap()
	for k := range unlimitedCounters {
		unlimitedCounters[k] = math.MaxInt32
	}

	invalidTxs := []*string{}
	invalidBlocksIndices := []int{}

	_this.txPool.MarkProcessedLimboDetails(size, invalidBlocksIndices, invalidTxs)
}
