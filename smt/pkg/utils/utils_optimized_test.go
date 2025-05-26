package utils

import (
	"strings"
	"testing"
)

func BenchmarkHashContractBytecodeComparison(b *testing.B) {
	// 测试不同长度的字节码
	testCases := []struct {
		name string
		data string
	}{
		{"small_100", strings.Repeat("e", 100)},
		{"medium_1000", strings.Repeat("e", 1000)},
		{"large_10000", strings.Repeat("e", 10000)},
		{"with_0x_prefix", "0x" + strings.Repeat("abcdef", 1000)},
		{"odd_length", strings.Repeat("a", 999)},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("original", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					HashContractBytecodeBigInt(tc.data)
				}
			})

			b.Run("optimized1", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					HashContractBytecodeBigIntOptimized(tc.data)
				}
			})

			b.Run("optimized2", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					HashContractBytecodeBigIntOptimized2(tc.data)
				}
			})

			b.Run("optimized3_cached", func(b *testing.B) {
				ClearHashCache() // 清理缓存
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					HashContractBytecodeBigIntOptimized3(tc.data)
				}
			})
		})
	}
}

// 验证优化版本的正确性
func TestHashContractBytecodeOptimizedCorrectness(t *testing.T) {
	testCases := []string{
		"e",
		"ee",
		"eee",
		strings.Repeat("e", 100),
		strings.Repeat("e", 1000),
		"0x" + strings.Repeat("abcdef", 100),
		strings.Repeat("a", 999), // 奇数长度
		"0x1234567890abcdef",
		"",
		"0x",
	}

	for i, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			original := HashContractBytecodeBigInt(tc)
			optimized1 := HashContractBytecodeBigIntOptimized(tc)
			optimized2 := HashContractBytecodeBigIntOptimized2(tc)

			ClearHashCache() // 清理缓存
			optimized3 := HashContractBytecodeBigIntOptimized3(tc)

			if original.Cmp(optimized1) != 0 {
				t.Errorf("Test case %d: optimized1 result differs from original\nOriginal: %s\nOptimized1: %s",
					i, original.String(), optimized1.String())
			}

			if original.Cmp(optimized2) != 0 {
				t.Errorf("Test case %d: optimized2 result differs from original\nOriginal: %s\nOptimized2: %s",
					i, original.String(), optimized2.String())
			}

			if original.Cmp(optimized3) != 0 {
				t.Errorf("Test case %d: optimized3 result differs from original\nOriginal: %s\nOptimized3: %s",
					i, original.String(), optimized3.String())
			}
		})
	}
}

func BenchmarkHexCharToValue(b *testing.B) {
	chars := []byte{'0', '5', '9', 'a', 'f', 'A', 'F'}

	b.Run("optimized", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, c := range chars {
				hexCharToValue(c)
			}
		}
	})
}
