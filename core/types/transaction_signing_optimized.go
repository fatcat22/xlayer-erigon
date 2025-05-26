package types

import (
	"errors"
	"sync"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/secp256k1"
)

// 预分配的对象池，减少内存分配
var (
	sigPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, crypto.SignatureLength)
		},
	}
)

// recoverPlainOptimized 合并所有优化的最佳版本
func recoverPlainOptimized(context *secp256k1.Context, sighash libcommon.Hash, R, S, Vb *uint256.Int, homestead bool) (libcommon.Address, error) {
	// 快速验证，避免不必要的计算
	if Vb.BitLen() > 8 {
		return libcommon.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)

	// 使用现有的签名验证函数，避免重复实现
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return libcommon.Address{}, ErrInvalidSig
	}

	// 从对象池获取签名缓冲区，减少内存分配
	sig := sigPool.Get().([]byte)
	defer sigPool.Put(sig)

	// 清零签名缓冲区
	for i := range sig {
		sig[i] = 0
	}

	// 优化的字节拷贝：直接操作字节
	rBytes := R.Bytes()
	sBytes := S.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	sig[64] = V

	// recover the public key from the signature
	pub, err := crypto.EcrecoverWithContext(context, sighash[:], sig)
	if err != nil {
		return libcommon.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return libcommon.Address{}, errors.New("invalid public key")
	}

	// 直接计算地址，避免额外的内存分配
	hash := crypto.Keccak256(pub[1:])
	var addr libcommon.Address
	copy(addr[:], hash[12:])
	return addr, nil
}

// 批量恢复优化：当需要恢复多个签名时
func recoverPlainBatch(context *secp256k1.Context, requests []RecoverRequest) ([]libcommon.Address, []error) {
	results := make([]libcommon.Address, len(requests))
	errs := make([]error, len(requests))

	// 预分配工作缓冲区
	sig := make([]byte, crypto.SignatureLength)

	for i, req := range requests {
		// 重置缓冲区
		for j := range sig {
			sig[j] = 0
		}

		if req.Vb.BitLen() > 8 {
			errs[i] = ErrInvalidSig
			continue
		}

		V := byte(req.Vb.Uint64() - 27)
		if !crypto.ValidateSignatureValues(V, req.R, req.S, req.Homestead) {
			errs[i] = ErrInvalidSig
			continue
		}

		// 编码签名
		r, s := req.R.Bytes(), req.S.Bytes()
		copy(sig[32-len(r):32], r)
		copy(sig[64-len(s):64], s)
		sig[64] = V

		// 恢复公钥
		pub, err := crypto.EcrecoverWithContext(context, req.Sighash[:], sig)
		if err != nil {
			errs[i] = err
			continue
		}
		if len(pub) == 0 || pub[0] != 4 {
			errs[i] = errors.New("invalid public key")
			continue
		}

		// 计算地址
		copy(results[i][:], crypto.Keccak256(pub[1:])[12:])
	}

	return results, errs
}

// 批量恢复请求结构
type RecoverRequest struct {
	Sighash   libcommon.Hash
	R, S, Vb  *uint256.Int
	Homestead bool
}
