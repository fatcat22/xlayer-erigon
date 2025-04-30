package common

import "github.com/linxGnu/grocksdb"

type RocksDBOptions struct {
	lruCache *grocksdb.Cache
	bbto     *grocksdb.BlockBasedTableOptions

	Opts   *grocksdb.Options
	TxOpts *grocksdb.TransactionDBOptions
}

func NewRocksDBOptions() *RocksDBOptions {
	lruCache := grocksdb.NewLRUCache(3 << 30)

	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(lruCache)

	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetBlockBasedTableFactory(bbto)

	return &RocksDBOptions{
		lruCache: lruCache,
		bbto:     bbto,
		Opts:     opts,
		TxOpts:   grocksdb.NewDefaultTransactionDBOptions(),
	}
}

func (o *RocksDBOptions) Destroy() {
	if o.lruCache != nil {
		o.lruCache.Destroy()
		o.lruCache = nil
	}

	if o.bbto != nil {
		o.bbto.Destroy()
		o.bbto = nil
	}

	if o.Opts != nil {
		o.Opts.Destroy()
		o.Opts = nil
	}

	if o.TxOpts != nil {
		o.TxOpts.Destroy()
		o.TxOpts = nil
	}
}
