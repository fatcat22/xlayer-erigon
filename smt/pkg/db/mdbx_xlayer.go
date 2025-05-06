package db

import (
	"github.com/benbjohnson/immutable"
	"github.com/ledgerwatch/erigon-lib/kv/membatch"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

func (m *EriDb) SetCache(smtCachedMapValue *immutable.Map[string, *immutable.Map[string, []byte]]) {
	if smtCachedMapValue == nil {
		smtCachedMapValue = immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	}

	mapCache, ok := m.tx.(*membatch.Mapmutation)
	if !ok {
		return // don't roll back a kvRw tx
	}

	cache := make(map[string]map[string][]byte, smtCachedMapValue.Len())
	// Convert smtCachedMapValue to cache
	outerIter := smtCachedMapValue.Iterator()
	for !outerIter.Done() {
		table, innerMap, _ := outerIter.Next()
		// Create inner map for this table
		innerCache := make(map[string][]byte, innerMap.Len())

		// Iterate over inner map
		innerIter := innerMap.Iterator()
		for !innerIter.Done() {
			key, value, _ := innerIter.Next()
			innerCache[key] = value
		}

		cache[table] = innerCache
	}

	mapCache.SetCache(cache)
}

func (m *EriDb) RetriveAndCleanCache() map[string]map[string][]byte {
	return nil
}

func (m *EriRoDb) GetLastHeight() (uint64, error) {
	data, err := m.kvTxRoSMT.GetOne(TableStats, []byte(MetaLastHeight))
	if err != nil {
		return 0, err
	}

	return utils.ConvertBytesToUint64(data)
}

func (m *EriDb) SetLastHeight(blockHeight uint64) error {
	v := utils.ConvertUint64ToBytes(blockHeight)
	return m.tx.Put(TableStats, []byte(MetaLastHeight), []byte(v))
}
