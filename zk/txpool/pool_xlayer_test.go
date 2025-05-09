package txpool

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/datadir"
	"github.com/ledgerwatch/erigon-lib/common/u256"
	"github.com/ledgerwatch/erigon-lib/gointerfaces"
	"github.com/ledgerwatch/erigon-lib/gointerfaces/remote"
	"github.com/ledgerwatch/erigon-lib/kv/kvcache"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/ledgerwatch/erigon-lib/kv/temporal/temporaltest"
	"github.com/ledgerwatch/erigon-lib/txpool/txpoolcfg"
	"github.com/ledgerwatch/erigon-lib/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test yield gas limit configuration.
// Add 100 normal txs and set the yield gas limit to 21_000 * 40 = 840_000.
// Ensure that yield txs call only gets 40 txs.
func TestAddLocalTxsWithYieldGasLimit(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	ch := make(chan types.Announcements, 100)
	_, coreDB, _ := temporaltest.NewTestDB(t, datadir.New(t.TempDir()))
	defer coreDB.Close()

	db := memdb.NewTestPoolDB(t)
	path := fmt.Sprintf("/tmp/db-test-%v", time.Now().UTC().Format(time.RFC3339Nano))
	txPoolDB := newTestTxPoolDB(t, path)
	defer txPoolDB.Close()
	aclsDB := newTestACLDB(t, path)
	defer aclsDB.Close()

	// Check if the dbs are created.
	require.NotNil(t, db)
	require.NotNil(t, txPoolDB)
	require.NotNil(t, aclsDB)

	cfg := txpoolcfg.DefaultConfig
	ethCfg := &ethconfig.Defaults
	sendersCache := kvcache.New(kvcache.DefaultCoherentConfig)

	// Create 100 normal addresses for testing
	var normalAddresses []common.Address
	for i := 0; i < 100; i++ {
		addr := common.HexToAddress(fmt.Sprintf("0x%x", i+10))
		normalAddresses = append(normalAddresses, addr)
	}

	// Set the yield gas limit to 40 transfer txs
	ethCfg.DeprecatedTxPool.YieldGasLimit = 840_000

	// Create a new txpool
	pool, err := New(ch, coreDB, cfg, ethCfg, sendersCache, *u256.N1, nil, nil, aclsDB)
	assert.NoError(err)
	require.True(pool != nil)
	ctx := context.Background()
	var stateVersionID uint64 = 0
	pendingBaseFee := uint64(200000)
	h1 := gointerfaces.ConvertHashToH256([32]byte{})

	change := &remote.StateChangeBatch{
		StateVersionId:      stateVersionID,
		PendingBlockBaseFee: pendingBaseFee,
		BlockGasLimit:       1000000,
		ChangeBatch: []*remote.StateChange{
			{BlockHeight: 0, BlockHash: h1},
		},
	}

	// Fund all addresses with 18 Ether for sending transactions
	v := make([]byte, types.EncodeSenderLengthForStorage(0, *uint256.NewInt(18 * common.Ether)))
	types.EncodeSender(0, *uint256.NewInt(18 * common.Ether), v)
	for _, addr := range normalAddresses {
		change.ChangeBatch[0].Changes = append(change.ChangeBatch[0].Changes, &remote.AccountChange{
			Action:  remote.Action_UPSERT,
			Address: gointerfaces.ConvertAddressToH160(addr),
			Data:    v,
		})
	}
	tx, err := db.BeginRw(ctx)
	require.NoError(err)
	defer tx.Rollback()
	err = pool.OnNewBlock(ctx, change, types.TxSlots{}, types.TxSlots{}, tx)
	assert.NoError(err)

	// Spam the pool and add 100 normal transactions
	var normalTxSlots types.TxSlots
	for i := 0; i < 100; i++ {
		txSlot := &types.TxSlot{
			Rlp:    []byte{byte(i)},
			Tip:    *uint256.NewInt(300000),
			FeeCap: *uint256.NewInt(1000000000),
			Gas:    21_000,
			Nonce:  0,
		}
		txSlot.IDHash[0] = byte(i)
		normalTxSlots.Append(txSlot, normalAddresses[i][:], true)
	}
	reasons, err := pool.AddLocalTxs(ctx, normalTxSlots, tx)
	assert.NoError(err)
	for _, reason := range reasons {
		assert.Equal(Success, reason, reason.String())
	}

	// since every transaction uses 21_000 intrinsic gas.
	slots := types.TxsRlp{}
	allConditionsOk, count, err := pool.YieldBest(100, &slots, tx, 0, 0, 0, mapset.NewSet[[32]byte]())
	assert.NoError(err)
	assert.True(allConditionsOk)

	// Check that only 40 transactions are yielded
	assert.Equal(40, count)
}

// Test okpay priority yield percentage configuration - more OkPay transactions than the reserved slots.
// Add 10 OkPay txs, 100 normal txs, with okpay priority yield percentage of 0.8.
// Set the yield gas limit to 21_000 * 10 = 210_000.
// Ensure that only 8 OkPay txs are obtained, and 2 normal txs are included.
func TestAddLocalTxsWithOkPayTxs1(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	ch := make(chan types.Announcements, 100)
	_, coreDB, _ := temporaltest.NewTestDB(t, datadir.New(t.TempDir()))
	defer coreDB.Close()

	db := memdb.NewTestPoolDB(t)
	path := fmt.Sprintf("/tmp/db-test-%v", time.Now().UTC().Format(time.RFC3339Nano))
	txPoolDB := newTestTxPoolDB(t, path)
	defer txPoolDB.Close()
	aclsDB := newTestACLDB(t, path)
	defer aclsDB.Close()

	// Check if the dbs are created.
	require.NotNil(t, db)
	require.NotNil(t, txPoolDB)
	require.NotNil(t, aclsDB)

	cfg := txpoolcfg.DefaultConfig
	ethCfg := &ethconfig.Defaults
	sendersCache := kvcache.New(kvcache.DefaultCoherentConfig)

	// Create 10 OkPay addresses for testing
	var okPayAddresses []common.Address
	for i := 0; i < 10; i++ {
		addr := common.HexToAddress(fmt.Sprintf("0x%x", i))
		okPayAddresses = append(okPayAddresses, addr)
	}

	// Create 100 normal addresses for testing
	var normalAddresses []common.Address
	for i := 0; i < 100; i++ {
		addr := common.HexToAddress(fmt.Sprintf("0x%x", i+10))
		normalAddresses = append(normalAddresses, addr)
	}

	// Set OkPay addresses and yield gas limit configs
	ethCfg.Zk.XLayer.OkPaySenderAccountsList = *common.NewOrderedListOfAddresses(len(okPayAddresses))
	for _, addr := range okPayAddresses {
		ethCfg.Zk.XLayer.OkPaySenderAccountsList.Add(addr)
	}
	ethCfg.Zk.XLayer.OkPaySenderAccountsList.Sort()
	ethCfg.DeprecatedTxPool.YieldGasLimit = 210_000

	// Create a new txpool
	pool, err := New(ch, coreDB, cfg, ethCfg, sendersCache, *u256.N1, nil, nil, aclsDB)
	assert.NoError(err)
	require.True(pool != nil)
	ctx := context.Background()
	var stateVersionID uint64 = 0
	pendingBaseFee := uint64(200000)
	h1 := gointerfaces.ConvertHashToH256([32]byte{})

	change := &remote.StateChangeBatch{
		StateVersionId:      stateVersionID,
		PendingBlockBaseFee: pendingBaseFee,
		BlockGasLimit:       1000000,
		ChangeBatch: []*remote.StateChange{
			{BlockHeight: 0, BlockHash: h1},
		},
	}

	// Fund all addresses with 18 Ether for sending transactions
	v := make([]byte, types.EncodeSenderLengthForStorage(0, *uint256.NewInt(18 * common.Ether)))
	types.EncodeSender(0, *uint256.NewInt(18 * common.Ether), v)

	for _, addr := range okPayAddresses {
		change.ChangeBatch[0].Changes = append(change.ChangeBatch[0].Changes, &remote.AccountChange{
			Action:  remote.Action_UPSERT,
			Address: gointerfaces.ConvertAddressToH160(addr),
			Data:    v,
		})
	}

	for _, addr := range normalAddresses {
		change.ChangeBatch[0].Changes = append(change.ChangeBatch[0].Changes, &remote.AccountChange{
			Action:  remote.Action_UPSERT,
			Address: gointerfaces.ConvertAddressToH160(addr),
			Data:    v,
		})
	}
	tx, err := db.BeginRw(ctx)
	require.NoError(err)
	defer tx.Rollback()
	err = pool.OnNewBlock(ctx, change, types.TxSlots{}, types.TxSlots{}, tx)
	assert.NoError(err)

	// Spam the pool and add 100 normal transactions
	var normalTxSlots types.TxSlots
	for i := 0; i < 100; i++ {
		txSlot := &types.TxSlot{
			Rlp:    []byte{byte(i)},
			Tip:    *uint256.NewInt(300000),
			FeeCap: *uint256.NewInt(1000000000),
			Gas:    21_000,
			Nonce:  0,
		}
		txSlot.IDHash[0] = byte(i)
		normalTxSlots.Append(txSlot, normalAddresses[i][:], true)
	}
	reasons, err := pool.AddLocalTxs(ctx, normalTxSlots, tx)
	assert.NoError(err)
	for _, reason := range reasons {
		assert.Equal(Success, reason, reason.String())
	}

	// Add 10 mock OkPay transactions to the pool
	var okPayTxSlots types.TxSlots
	for i := 0; i < 10; i++ {
		txSlot := &types.TxSlot{
			Rlp:    []byte{byte(i + 100)},
			Tip:    *uint256.NewInt(300000),
			FeeCap: *uint256.NewInt(1000000000),
			Gas:    21_000,
			Nonce:  0,
		}
		txSlot.IDHash[0] = byte(i + 100)
		okPayTxSlots.Append(txSlot, okPayAddresses[i][:], true)
	}
	reasons, err = pool.AddLocalTxs(ctx, okPayTxSlots, tx)
	assert.NoError(err)
	for _, reason := range reasons {
		assert.Equal(Success, reason, reason.String())
	}

	slots := types.TxsRlp{}
	allConditionsOk, count, err := pool.YieldBest(20, &slots, tx, 0, 0, 0, mapset.NewSet[[32]byte]())
	assert.NoError(err)
	assert.True(allConditionsOk)

	// Check that 10 transactions are yielded, and normal transactions are included as well
	assert.Equal(10, count)

	// Check only all the OkPay transactions were included
	okPayCount := 0
	for _, rlpTx := range slots.Txs {
		for _, okPayTx := range okPayTxSlots.Txs {
			if bytes.Equal(rlpTx, okPayTx.Rlp) {
				okPayCount++
			}
		}
	}
	assert.Equal(8, okPayCount)
}

// Test okpay priority yield percentage configuration - more OkPay transactions than the reserved slots,
// but not many normal txs.
// Add 9 OkPay txs, 1 normal txs, with okpay priority yield percentage of 0.8.
// Set the yield gas limit to 21_000 * 10 = 210_000.
// All txs should be yielded - yield logic should be able to still include okpay txs when there are no
// more normal txs to yield.
func TestAddLocalTxsWithOkPayTxs2(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	ch := make(chan types.Announcements, 100)
	_, coreDB, _ := temporaltest.NewTestDB(t, datadir.New(t.TempDir()))
	defer coreDB.Close()

	db := memdb.NewTestPoolDB(t)
	path := fmt.Sprintf("/tmp/db-test-%v", time.Now().UTC().Format(time.RFC3339Nano))
	txPoolDB := newTestTxPoolDB(t, path)
	defer txPoolDB.Close()
	aclsDB := newTestACLDB(t, path)
	defer aclsDB.Close()

	// Check if the dbs are created.
	require.NotNil(t, db)
	require.NotNil(t, txPoolDB)
	require.NotNil(t, aclsDB)

	cfg := txpoolcfg.DefaultConfig
	ethCfg := &ethconfig.Defaults
	sendersCache := kvcache.New(kvcache.DefaultCoherentConfig)

	// Create 1 normal addresses for testing
	var normalAddresses []common.Address
	for i := 0; i < 1; i++ {
		addr := common.HexToAddress(fmt.Sprintf("0x%x", i+10))
		normalAddresses = append(normalAddresses, addr)
	}

	// Create 9 OkPay addresses for testing
	var okPayAddresses []common.Address
	for i := 0; i < 10; i++ {
		addr := common.HexToAddress(fmt.Sprintf("0x%x", i))
		okPayAddresses = append(okPayAddresses, addr)
	}

	// Set OkPay addresses and yield gas limit configs
	ethCfg.Zk.XLayer.OkPaySenderAccountsList = *common.NewOrderedListOfAddresses(len(okPayAddresses))
	for _, addr := range okPayAddresses {
		ethCfg.Zk.XLayer.OkPaySenderAccountsList.Add(addr)
	}
	ethCfg.Zk.XLayer.OkPaySenderAccountsList.Sort()
	ethCfg.DeprecatedTxPool.YieldGasLimit = 210_000

	// Create a new txpool
	pool, err := New(ch, coreDB, cfg, ethCfg, sendersCache, *u256.N1, nil, nil, aclsDB)
	assert.NoError(err)
	require.True(pool != nil)
	ctx := context.Background()
	var stateVersionID uint64 = 0
	pendingBaseFee := uint64(200000)
	h1 := gointerfaces.ConvertHashToH256([32]byte{})

	change := &remote.StateChangeBatch{
		StateVersionId:      stateVersionID,
		PendingBlockBaseFee: pendingBaseFee,
		BlockGasLimit:       1000000,
		ChangeBatch: []*remote.StateChange{
			{BlockHeight: 0, BlockHash: h1},
		},
	}

	// Fund all addresses with 18 Ether for sending transactions
	v := make([]byte, types.EncodeSenderLengthForStorage(0, *uint256.NewInt(18 * common.Ether)))
	types.EncodeSender(0, *uint256.NewInt(18 * common.Ether), v)

	for _, addr := range okPayAddresses {
		change.ChangeBatch[0].Changes = append(change.ChangeBatch[0].Changes, &remote.AccountChange{
			Action:  remote.Action_UPSERT,
			Address: gointerfaces.ConvertAddressToH160(addr),
			Data:    v,
		})
	}

	for _, addr := range normalAddresses {
		change.ChangeBatch[0].Changes = append(change.ChangeBatch[0].Changes, &remote.AccountChange{
			Action:  remote.Action_UPSERT,
			Address: gointerfaces.ConvertAddressToH160(addr),
			Data:    v,
		})
	}

	tx, err := db.BeginRw(ctx)
	require.NoError(err)
	defer tx.Rollback()
	err = pool.OnNewBlock(ctx, change, types.TxSlots{}, types.TxSlots{}, tx)
	assert.NoError(err)

	// Add 1 normal transactions
	var normalTxSlots types.TxSlots
	for i := 0; i < 1; i++ {
		txSlot := &types.TxSlot{
			Rlp:    []byte{byte(i)},
			Tip:    *uint256.NewInt(300000),
			FeeCap: *uint256.NewInt(1000000000),
			Gas:    21_000,
			Nonce:  0,
		}
		txSlot.IDHash[0] = byte(i)
		normalTxSlots.Append(txSlot, normalAddresses[i][:], true)
	}
	reasons, err := pool.AddLocalTxs(ctx, normalTxSlots, tx)
	assert.NoError(err)
	for _, reason := range reasons {
		assert.Equal(Success, reason, reason.String())
	}

	// Add 9 mock OkPay transactions to the pool
	var okPayTxSlots types.TxSlots
	for i := 0; i < 9; i++ {
		txSlot := &types.TxSlot{
			Rlp:    []byte{byte(i + 1)},
			Tip:    *uint256.NewInt(300000),
			FeeCap: *uint256.NewInt(1000000000),
			Gas:    21_000,
			Nonce:  0,
		}
		txSlot.IDHash[0] = byte(i + 1)
		okPayTxSlots.Append(txSlot, okPayAddresses[i][:], true)
	}
	reasons, err = pool.AddLocalTxs(ctx, okPayTxSlots, tx)
	assert.NoError(err)
	for _, reason := range reasons {
		assert.Equal(Success, reason, reason.String())
	}

	slots := types.TxsRlp{}
	allConditionsOk, count, err := pool.YieldBest(20, &slots, tx, 0, 0, 0, mapset.NewSet[[32]byte]())
	assert.NoError(err)
	assert.True(allConditionsOk)

	// Check that 10 transactions are yielded, and normal transactions are included as well
	assert.Equal(10, count)

	// Check only all the OkPay transactions were included
	okPayCount := 0
	for _, rlpTx := range slots.Txs {
		for _, okPayTx := range okPayTxSlots.Txs {
			if bytes.Equal(rlpTx, okPayTx.Rlp) {
				okPayCount++
			}
		}
	}
	assert.Equal(9, okPayCount)
}
