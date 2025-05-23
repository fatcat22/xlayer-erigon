package db

import (
	"context"
	"encoding/hex"
	"math/big"

	"fmt"
	"strings"

	"github.com/ledgerwatch/erigon-lib/etl"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/membatch"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/log/v3"
)

type SmtDbTx interface {
	GetOne(bucket string, key []byte) ([]byte, error)
	Put(bucket string, key []byte, value []byte) error
	Has(bucket string, key []byte) (bool, error)
	Delete(bucket string, key []byte) error
	ForEach(bucket string, start []byte, fn func(k, v []byte) error) error
	ForPrefix(bucket string, prefix []byte, fn func(k, v []byte) error) error
	Commit() error
	Rollback()
}

const TableSmt = "HermezSmt"
const TableStats = "HermezSmtStats"
const TableAccountValues = "HermezSmtAccountValues"
const TableMetadata = "HermezSmtMetadata"
const TableHashKey = "HermezSmtHashKey"

const MetaLastRoot = "lastRoot"
const MetaDepth = "depth"
const MetaLastHeight = "lastHeight" // For X Layer, split db and ac

var HermezSmtTables = []string{TableSmt, TableStats, TableAccountValues, TableMetadata, TableHashKey}

// For X Layer, split db and ac
type EriDb struct {
	kvTxSMT     kv.RwTx
	tx          SmtDbTx
	kvTxChainDB kv.RwTx
	*EriRoDb
	accountCollector   *etl.Collector
	keySourceCollector *etl.Collector
	hashKeyCollector   *etl.Collector
	smtCollector       *etl.Collector
}

type EriRoDb struct {
	kvTxRoSMT     kv.Getter
	kvTxRoChainDB kv.Getter
}

func CreateEriDbBuckets(tx kv.RwTx) error {
	// For X Layer, split db and ac
	for _, table := range HermezSmtTables {
		err := tx.CreateBucket(table)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewEriDb(txsmt kv.RwTx, txcdb kv.RwTx) *EriDb {
	// For X Layer, split db and ac
	var tx kv.RwTx = txsmt
	if tx == nil {
		tx = txcdb
	}

	logger := log.New()

	// make etls for holding account data and key sources
	accountCollector := etl.NewCollector("", "./account_collector", etl.NewSortableBuffer(etl.BufferOptimalSize), logger)
	accountCollector.LogLvl(log.LvlTrace)

	keySourceCollector := etl.NewCollector("", "./key_source_collector", etl.NewSortableBuffer(etl.BufferOptimalSize), logger)
	keySourceCollector.LogLvl(log.LvlTrace)

	hashKeyCollector := etl.NewCollector("", "./hash_key_collector", etl.NewSortableBuffer(etl.BufferOptimalSize), logger)
	hashKeyCollector.LogLvl(log.LvlTrace)

	smtCollector := etl.NewCollector("", "./smt_collector", etl.NewSortableBuffer(etl.BufferOptimalSize), logger)
	smtCollector.LogLvl(log.LvlTrace)

	return &EriDb{
		tx:                 tx,
		kvTxSMT:            tx,
		kvTxChainDB:        txcdb,
		accountCollector:   accountCollector,
		keySourceCollector: keySourceCollector,
		hashKeyCollector:   hashKeyCollector,
		smtCollector:       smtCollector,
		EriRoDb:            NewRoEriDb(txsmt, txcdb),
	}
}

func NewRoEriDb(txsmt, txcdb kv.Getter) *EriRoDb {
	// For X Layer, split db and ac
	var tx kv.Getter = txsmt
	if tx == nil {
		tx = txcdb
	}
	return &EriRoDb{
		kvTxRoSMT:     tx,
		kvTxRoChainDB: txcdb,
	}
}

func (m *EriDb) CloseAccountCollectors() {
	m.accountCollector.Close()
	m.keySourceCollector.Close()
}

func (m *EriDb) CloseSmtCollectors() {
	m.hashKeyCollector.Close()
	m.smtCollector.Close()
}

func (m *EriDb) LoadAccountCollectors() error {
	err := m.accountCollector.Load(m.kvTxSMT, "", func(k, v []byte, _ etl.CurrentTableReader, next etl.LoadNextFunc) error {
		m.tx.Put(TableAccountValues, k, v)
		return nil
	}, etl.TransformArgs{})
	if err != nil {
		return err
	}

	err = m.keySourceCollector.Load(m.kvTxSMT, "", func(k, v []byte, _ etl.CurrentTableReader, next etl.LoadNextFunc) error {
		m.tx.Put(TableMetadata, k, v)
		return nil
	}, etl.TransformArgs{})
	if err != nil {
		return err
	}

	return nil
}

func (m *EriDb) LoadSmtCollectors() error {
	err := m.hashKeyCollector.Load(m.kvTxSMT, "", func(k, v []byte, _ etl.CurrentTableReader, next etl.LoadNextFunc) error {
		m.tx.Put(TableHashKey, k, v)
		return nil
	}, etl.TransformArgs{})
	if err != nil {
		return err
	}

	err = m.smtCollector.Load(m.kvTxSMT, "", func(k, v []byte, _ etl.CurrentTableReader, next etl.LoadNextFunc) error {
		m.tx.Put(TableSmt, k, v)
		return nil
	}, etl.TransformArgs{})
	if err != nil {
		return err
	}

	return nil
}

func (m *EriDb) CollectAccountValue(key utils.NodeKey, value utils.NodeValue8) {
	k := key.ToHex()
	v := value.ToHex()

	m.accountCollector.Collect([]byte(k), []byte(v))
}

func (m *EriDb) CollectKeySource(key utils.NodeKey, value []byte) {
	keyBytes := utils.ArrayToBytes(key[:])
	m.keySourceCollector.Collect(keyBytes, value)
}

func (m *EriDb) CollectHashKey(key utils.NodeKey, value utils.NodeKey) {
	keyBytes := utils.ArrayToBytes(key[:])
	valBytes := utils.ArrayToBytes(value[:])
	m.hashKeyCollector.Collect(keyBytes, valBytes)
}

func (m *EriDb) CollectSmt(key utils.NodeKey, value utils.NodeValue12) {
	k := key.ToHex()
	v := value.ToHex()

	m.smtCollector.Collect([]byte(k), []byte(v))
}

func (m *EriDb) OpenBatch(quitCh <-chan struct{}) {
	batch := membatch.NewHashBatch(m.kvTxSMT, quitCh, "./tempdb", log.New())
	defer func() {
		batch.Close()
	}()
	m.tx = batch
	m.kvTxRoSMT = batch
}

func (m *EriDb) CommitBatch() error {
	batch, ok := m.tx.(kv.PendingMutations)
	if !ok {
		return nil // don't roll back a kvRw tx
	}
	// err := m.tx.Commit()
	err := batch.Flush(context.Background(), m.kvTxSMT)
	if err != nil {
		// m.tx.Rollback()
		batch.Close()
		return err
	}
	m.tx = m.kvTxSMT
	m.kvTxRoSMT = m.kvTxSMT
	return nil
}

func (m *EriDb) RollbackBatch() {
	if m.tx == nil {
		return
	}
	if _, ok := m.tx.(kv.PendingMutations); !ok {
		return // don't roll back a kvRw tx
	}
	m.tx.(kv.PendingMutations).Close()
	m.tx = m.kvTxSMT
	m.kvTxRoSMT = m.kvTxSMT
}

func (m *EriRoDb) GetLastRoot() (*big.Int, error) {
	data, err := m.kvTxRoSMT.GetOne(TableStats, []byte(MetaLastRoot))
	if err != nil {
		return big.NewInt(0), err
	}

	if data == nil {
		return big.NewInt(0), nil
	}

	return utils.ConvertHexToBigInt(string(data)), nil
}

func (m *EriDb) SetLastRoot(r *big.Int) error {
	v := utils.ConvertBigIntToHex(r)
	return m.tx.Put(TableStats, []byte(MetaLastRoot), []byte(v))
}

func (m *EriRoDb) GetDepth() (uint8, error) {
	data, err := m.kvTxRoSMT.GetOne(TableStats, []byte(MetaDepth))
	if err != nil {
		return 0, err
	}

	if data == nil {
		return 0, nil
	}

	return data[0], nil
}

func (m *EriDb) SetDepth(depth uint8) error {
	return m.tx.Put(TableStats, []byte(MetaDepth), []byte{depth})
}

func (m *EriRoDb) Get(key utils.NodeKey) (utils.NodeValue12, error) {
	k := key.ToHex()

	data, err := m.kvTxRoSMT.GetOne(TableSmt, []byte(k))
	if err != nil {
		return utils.NodeValue12{}, err
	}

	if data == nil {
		return utils.NodeValue12{}, nil
	}

	vConc := utils.ConvertHexToBigInt(string(data))
	val := utils.ScalarToNodeValue(vConc)

	return val, nil
}

func (m *EriDb) Insert(key utils.NodeKey, value utils.NodeValue12) error {
	k := key.ToHex()
	v := value.ToHex()

	return m.tx.Put(TableSmt, []byte(k), []byte(v))
}

func (m *EriDb) Delete(key string) error {
	return m.tx.Delete(TableSmt, []byte(key))
}

func (m *EriDb) DeleteByNodeKey(key utils.NodeKey) error {
	k := key.ToHex()
	return m.tx.Delete(TableSmt, []byte(k))
}

func (m *EriRoDb) GetAccountValue(key utils.NodeKey) (utils.NodeValue8, error) {
	k := key.ToHex()

	// For X Layer, split db and ac
	data, err := m.kvTxRoSMT.GetOne(TableAccountValues, []byte(k))
	if err != nil {
		return utils.NodeValue8{}, err
	}

	if data == nil {
		return utils.NodeValue8{}, nil
	}

	vConc := utils.ConvertHexToBigInt(string(data))
	val := utils.ScalarToNodeValue8(vConc)

	return val, nil
}

func (m *EriDb) InsertAccountValue(key utils.NodeKey, value utils.NodeValue8) error {
	k := key.ToHex()
	v := value.ToHex()

	return m.tx.Put(TableAccountValues, []byte(k), []byte(v))
}

func (m *EriDb) InsertKeySource(key utils.NodeKey, value []byte) error {
	keyBytes := utils.ArrayToBytes(key[:])

	return m.tx.Put(TableMetadata, keyBytes, value)
}

func (m *EriDb) DeleteKeySource(key utils.NodeKey) error {
	keyBytes := utils.ArrayToBytes(key[:])

	return m.tx.Delete(TableMetadata, keyBytes)
}

func (m *EriRoDb) GetKeySource(key utils.NodeKey) ([]byte, error) {
	keyBytes := utils.ArrayToBytes(key[:])

	// For X Layer, split db and ac
	data, err := m.kvTxRoSMT.GetOne(TableMetadata, keyBytes)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrNotFound
	}

	return data, nil
}

func (m *EriDb) InsertHashKey(key utils.NodeKey, value utils.NodeKey) error {
	keyBytes := utils.ArrayToBytes(key[:])
	valBytes := utils.ArrayToBytes(value[:])

	return m.tx.Put(TableHashKey, keyBytes, valBytes)
}

func (m *EriDb) DeleteHashKey(key utils.NodeKey) error {
	keyBytes := utils.ArrayToBytes(key[:])
	return m.tx.Delete(TableHashKey, keyBytes)
}

func (m *EriRoDb) GetHashKey(key utils.NodeKey) (utils.NodeKey, error) {
	keyBytes := utils.ArrayToBytes(key[:])

	// For X Layer, split db and ac
	data, err := m.kvTxRoSMT.GetOne(TableHashKey, keyBytes)
	if err != nil {
		return utils.NodeKey{}, err
	}

	if data == nil {
		return utils.NodeKey{}, fmt.Errorf("hash key %x not found", keyBytes)
	}

	nv := big.NewInt(0).SetBytes(data)

	na := utils.ScalarToArray(nv)

	return utils.NodeKey{na[0], na[1], na[2], na[3]}, nil
}

func (m *EriRoDb) GetCode(codeHash []byte) ([]byte, error) {
	codeHash = utils.ResizeHashTo32BytesByPrefixingWithZeroes(codeHash)

	// For X Layer, split db and ac
	data, err := m.kvTxRoChainDB.GetOne(kv.Code, codeHash)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fmt.Errorf("code hash %x not found", codeHash)
	}

	return data, nil
}

func (m *EriDb) AddCode(code []byte) error {
	codeHash := utils.HashContractBytecode(hex.EncodeToString(code))

	codeHashBytes, err := hex.DecodeString(strings.TrimPrefix(codeHash, "0x"))
	if err != nil {
		return err
	}

	codeHashBytes = utils.ResizeHashTo32BytesByPrefixingWithZeroes(codeHashBytes)

	// For X Layer, split db and ac
	return m.kvTxChainDB.Put(kv.Code, codeHashBytes, code)
}

func (m *EriRoDb) PrintDb() {
	// For X Layer, split db and ac
	err := m.kvTxRoSMT.ForEach(TableSmt, []byte{}, func(k, v []byte) error {
		println(string(k), string(v))
		return nil
	})
	if err != nil {
		println(err)
	}
}

func (m *EriRoDb) GetDb() map[string][]string {
	transformedDb := make(map[string][]string)

	// For X Layer, split db and ac
	err := m.kvTxRoSMT.ForEach(TableSmt, []byte{}, func(k, v []byte) error {
		hk := string(k)

		vConc := utils.ConvertHexToBigInt(string(v))
		val := utils.ScalarToNodeValue(vConc)

		truncationLength := 12

		allFirst8PaddedWithZeros := true
		for i := 0; i < 8; i++ {
			if !strings.HasPrefix(fmt.Sprintf("%016x", val[i]), "00000000") {
				allFirst8PaddedWithZeros = false
				break
			}
		}

		if allFirst8PaddedWithZeros {
			truncationLength = 8
		}

		outputArr := make([]string, truncationLength)
		for i := 0; i < truncationLength; i++ {
			if i < len(val) {
				outputArr[i] = fmt.Sprintf("%016x", val[i])
			} else {
				outputArr[i] = "0000000000000000"
			}
		}

		transformedDb[hk] = outputArr
		return nil
	})

	if err != nil {
		log.Error(err.Error())
	}

	return transformedDb
}
