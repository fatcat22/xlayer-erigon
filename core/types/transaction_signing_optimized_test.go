package types

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/secp256k1"
)

// 生成测试数据
func generateTestSignature() (libcommon.Hash, *uint256.Int, *uint256.Int, *uint256.Int) {
	// 生成私钥
	privateKey, _ := ecdsa.GenerateKey(crypto.S256(), rand.Reader)

	// 生成消息哈希
	hash := crypto.Keccak256Hash([]byte("test message"))

	// 签名
	sig, _ := crypto.Sign(hash[:], privateKey)

	// 解析签名
	r := new(uint256.Int).SetBytes(sig[:32])
	s := new(uint256.Int).SetBytes(sig[32:64])
	v := new(uint256.Int).SetBytes([]byte{sig[64] + 27})

	return hash, r, s, v
}

func BenchmarkRecoverPlainComparison(b *testing.B) {
	hash, r, s, v := generateTestSignature()
	context := secp256k1.ContextForThread(0)

	b.Run("original", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = recoverPlain(context, hash, r, s, v, true)
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = recoverPlainOptimized(context, hash, r, s, v, true)
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = recoverPlainOptimized(context, hash, r, s, v, true)
		}
	})
}

func BenchmarkRecoverPlainBatch(b *testing.B) {
	context := secp256k1.ContextForThread(0)

	// 生成批量测试数据
	batchSizes := []int{1, 10, 100, 1000}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			requests := make([]RecoverRequest, size)
			for i := 0; i < size; i++ {
				hash, r, s, v := generateTestSignature()
				requests[i] = RecoverRequest{
					Sighash:   hash,
					R:         r,
					S:         s,
					Vb:        v,
					Homestead: true,
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = recoverPlainBatch(context, requests)
			}
		})

		b.Run(fmt.Sprintf("individual_%d", size), func(b *testing.B) {
			requests := make([]RecoverRequest, size)
			for i := 0; i < size; i++ {
				hash, r, s, v := generateTestSignature()
				requests[i] = RecoverRequest{
					Sighash:   hash,
					R:         r,
					S:         s,
					Vb:        v,
					Homestead: true,
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, req := range requests {
					_, _ = recoverPlainOptimized(context, req.Sighash, req.R, req.S, req.Vb, req.Homestead)
				}
			}
		})
	}
}

// 验证优化版本的正确性
func TestRecoverPlainOptimizedCorrectness(t *testing.T) {
	context := secp256k1.ContextForThread(0)

	// 生成多个测试用例
	for i := 0; i < 100; i++ {
		hash, r, s, v := generateTestSignature()

		original, err1 := recoverPlain(context, hash, r, s, v, true)
		optimized, err2 := recoverPlainOptimized(context, hash, r, s, v, true)

		if err1 != nil || err2 != nil {
			t.Fatalf("Test case %d: unexpected error - original: %v, optimized: %v",
				i, err1, err2)
		}

		if original != optimized {
			t.Errorf("Test case %d: optimized result differs from original\nOriginal: %s\nOptimized: %s",
				i, original.Hex(), optimized.Hex())
		}
	}
}

func TestRecoverPlainBatchCorrectness(t *testing.T) {
	context := secp256k1.ContextForThread(0)

	// 生成批量测试数据
	size := 50
	requests := make([]RecoverRequest, size)
	expectedResults := make([]libcommon.Address, size)

	for i := 0; i < size; i++ {
		hash, r, s, v := generateTestSignature()
		requests[i] = RecoverRequest{
			Sighash:   hash,
			R:         r,
			S:         s,
			Vb:        v,
			Homestead: true,
		}

		// 计算期望结果
		addr, err := recoverPlain(context, hash, r, s, v, true)
		if err != nil {
			t.Fatalf("Failed to generate expected result for test case %d: %v", i, err)
		}
		expectedResults[i] = addr
	}

	// 测试批量恢复
	results, errs := recoverPlainBatch(context, requests)

	for i := 0; i < size; i++ {
		if errs[i] != nil {
			t.Errorf("Batch recovery failed for test case %d: %v", i, errs[i])
			continue
		}

		if results[i] != expectedResults[i] {
			t.Errorf("Batch result differs for test case %d\nExpected: %s\nGot: %s",
				i, expectedResults[i].Hex(), results[i].Hex())
		}
	}
}

// 测试不同优化版本的性能差异
func TestRecoverPlainPerformanceComparison(t *testing.T) {
	context := secp256k1.ContextForThread(0)
	hash, r, s, v := generateTestSignature()

	// 测试所有版本都能正确工作
	original, err1 := recoverPlain(context, hash, r, s, v, true)
	if err1 != nil {
		t.Fatalf("Original version failed: %v", err1)
	}

	optimized, err2 := recoverPlainOptimized(context, hash, r, s, v, true)
	if err2 != nil {
		t.Fatalf("Optimized version failed: %v", err2)
	}

	// 验证两个版本结果一致
	if original != optimized {
		t.Errorf("Results differ between versions\nOriginal: %s\nOptimized: %s",
			original.Hex(), optimized.Hex())
	}
}

// 真实场景基准测试：处理大量不同的交易签名
func BenchmarkRecoverPlainRealWorld(b *testing.B) {
	context := secp256k1.ContextForThread(0)

	// 预生成大量不同的签名数据，模拟真实区块链环境
	const numSignatures = 1000
	signatures := make([]struct {
		hash    libcommon.Hash
		r, s, v *uint256.Int
	}, numSignatures)

	for i := 0; i < numSignatures; i++ {
		hash, r, s, v := generateTestSignature()
		signatures[i] = struct {
			hash    libcommon.Hash
			r, s, v *uint256.Int
		}{hash, r, s, v}
	}

	b.Run("original_real_world", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sig := &signatures[i%numSignatures]
			_, _ = recoverPlain(context, sig.hash, sig.r, sig.s, sig.v, true)
		}
	})

	b.Run("optimized_real_world", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sig := &signatures[i%numSignatures]
			_, _ = recoverPlainOptimized(context, sig.hash, sig.r, sig.s, sig.v, true)
		}
	})
}

// 批量处理基准测试
func BenchmarkRecoverPlainBatchProcessing(b *testing.B) {
	context := secp256k1.ContextForThread(0)

	batchSizes := []int{10, 50, 100, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_size_%d", batchSize), func(b *testing.B) {
			// 生成批量数据
			requests := make([]RecoverRequest, batchSize)
			for i := 0; i < batchSize; i++ {
				hash, r, s, v := generateTestSignature()
				requests[i] = RecoverRequest{
					Sighash:   hash,
					R:         r,
					S:         s,
					Vb:        v,
					Homestead: true,
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = recoverPlainBatch(context, requests)
			}
		})
	}
}
