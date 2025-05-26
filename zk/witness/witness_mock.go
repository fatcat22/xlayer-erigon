package witness

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type MockWitnessGenerator struct{}

func (g *MockWitnessGenerator) GetWitnessByBadBatch(tx kv.Tx, txsmt kv.Tx, ctx context.Context, batchNum uint64, debug, witnessFull bool) (witness []byte, err error) {
	return nil, nil
}

func (g *MockWitnessGenerator) GetWitnessByBlockRange(tx kv.Tx, txsmt kv.Tx, ctx context.Context, startBlock, endBlock uint64, debug, witnessFull bool, cache map[string]map[string][]byte) ([]byte, error) {
	return nil, nil
}
