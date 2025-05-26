package utils

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// 预分配的对象池，减少内存分配
var (
	bigIntPool = sync.Pool{
		New: func() interface{} {
			return new(big.Int)
		},
	}
	uint64SlicePool = sync.Pool{
		New: func() interface{} {
			return make([]uint64, 0, 16) // 预分配容量
		},
	}
	hashCache    = sync.Map{} // 使用sync.Map作为线程安全的缓存
	maxCacheSize = 1000       // 限制缓存大小
	cacheSize    = int64(0)   // 当前缓存大小
)

// 优化版本1：减少字符串操作和内存分配
func HashContractBytecodeBigIntOptimized(bc string) *big.Int {
	// 预处理：去除0x前缀
	bytecode := bc
	if len(bc) >= 2 && bc[0] == '0' && (bc[1] == 'x' || bc[1] == 'X') {
		bytecode = bc[2:]
	}

	// 如果长度为奇数，前面补0
	if len(bytecode)&1 != 0 {
		bytecode = "0" + bytecode
	}

	// 预计算所需的容量，减少字符串重新分配
	originalLen := len(bytecode)
	paddingNeeded := (56*2 - (originalLen+2)%(56*2)) % (56 * 2)
	finalLen := originalLen + 2 + paddingNeeded

	// 使用strings.Builder减少字符串拼接开销
	var builder strings.Builder
	builder.Grow(finalLen) // 预分配容量
	builder.WriteString(bytecode)
	builder.WriteString("01")

	// 批量添加填充字节
	for i := 0; i < paddingNeeded; i += 2 {
		builder.WriteString("00")
	}

	bytecode = builder.String()

	// 设置最后一个字节
	lastByteInt, _ := strconv.ParseInt(bytecode[len(bytecode)-2:], 16, 64)
	lastByte := strconv.FormatInt(lastByteInt|0x80, 16)
	if len(lastByte) == 1 {
		lastByte = "0" + lastByte
	}

	// 直接修改最后两个字符
	bytecode = bytecode[:len(bytecode)-2] + lastByte

	numBytes := float64(len(bytecode)) / 2
	numHashes := int(math.Ceil(numBytes / (BYTECODE_ELEMENTS_HASH * BYTECODE_BYTES_ELEMENT)))

	tmpHash := [4]uint64{0, 0, 0, 0}
	bytesPointer := 0

	maxBytesToAdd := BYTECODE_ELEMENTS_HASH * BYTECODE_BYTES_ELEMENT

	// 从对象池获取slice，减少内存分配
	elementsToHash := uint64SlicePool.Get().([]uint64)
	defer uint64SlicePool.Put(elementsToHash[:0]) // 归还时重置长度

	var in [8]uint64
	var capacity [4]uint64

	// 从对象池获取big.Int，减少内存分配
	scalar := bigIntPool.Get().(*big.Int)
	defer bigIntPool.Put(scalar)
	tmpScalar := bigIntPool.Get().(*big.Int)
	defer bigIntPool.Put(tmpScalar)

	for i := 0; i < numHashes; i++ {
		// 重置slice而不是重新分配
		elementsToHash = elementsToHash[:0]
		elementsToHash = append(elementsToHash, tmpHash[:]...)

		endPos := bytesPointer + maxBytesToAdd*2
		if endPos > len(bytecode) {
			endPos = len(bytecode)
		}
		subsetBytecode := bytecode[bytesPointer:endPos]
		bytesPointer += maxBytesToAdd * 2

		// 优化内层循环：减少字符串操作
		tmpElem := uint64(0)
		counter := 0
		shift := uint64(0)

		for j := 0; j < maxBytesToAdd; j++ {
			var byteVal uint64
			if j*2+1 < len(subsetBytecode) {
				// 直接解析两个字符为字节值
				b1 := hexCharToValue(subsetBytecode[j*2])
				b2 := hexCharToValue(subsetBytecode[j*2+1])
				byteVal = uint64(b1<<4 | b2)
			}

			tmpElem |= byteVal << shift
			shift += 8
			counter++

			if counter == BYTECODE_BYTES_ELEMENT {
				elementsToHash = append(elementsToHash, tmpElem)
				tmpElem = 0
				shift = 0
				counter = 0
			}
		}

		copy(in[:], elementsToHash[4:12])
		copy(capacity[:], elementsToHash[:4])

		tmpHash = Hash(in, capacity)
	}

	return ArrayToScalar(tmpHash[:])
}

// 内联函数：将十六进制字符转换为数值
func hexCharToValue(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

// 优化版本2：使用更高效的字节处理（修复版本）
func HashContractBytecodeBigIntOptimized2(bc string) *big.Int {
	// 使用与原版本相同的逻辑，但优化字符串操作
	bytecode := bc
	if len(bc) >= 2 && bc[0] == '0' && (bc[1] == 'x' || bc[1] == 'X') {
		bytecode = bc[2:]
	}

	// 如果长度为奇数，前面补0
	if len(bytecode)&1 != 0 {
		bytecode = "0" + bytecode
	}

	// 预计算所需的容量
	originalLen := len(bytecode)
	paddingNeeded := (56*2 - (originalLen+2)%(56*2)) % (56 * 2)

	// 使用字节数组而不是字符串拼接
	totalLen := originalLen + 2 + paddingNeeded
	result := make([]byte, totalLen)

	// 复制原始字节码
	copy(result, bytecode)

	// 添加"01"
	result[originalLen] = '0'
	result[originalLen+1] = '1'

	// 添加填充的"00"
	for i := originalLen + 2; i < totalLen; i += 2 {
		result[i] = '0'
		result[i+1] = '0'
	}

	// 转换回字符串进行最后的处理
	bytecode = string(result)

	// 设置最后一个字节
	lastByteInt, _ := strconv.ParseInt(bytecode[len(bytecode)-2:], 16, 64)
	lastByte := strconv.FormatInt(lastByteInt|0x80, 16)
	if len(lastByte) == 1 {
		lastByte = "0" + lastByte
	}
	bytecode = bytecode[:len(bytecode)-2] + lastByte

	numBytes := float64(len(bytecode)) / 2
	numHashes := int(math.Ceil(numBytes / (BYTECODE_ELEMENTS_HASH * BYTECODE_BYTES_ELEMENT)))

	tmpHash := [4]uint64{0, 0, 0, 0}
	bytesPointer := 0

	maxBytesToAdd := BYTECODE_ELEMENTS_HASH * BYTECODE_BYTES_ELEMENT
	var elementsToHash []uint64
	var in [8]uint64
	var capacity [4]uint64

	// 预分配slice
	elementsToHash = make([]uint64, 0, 12)

	for i := 0; i < numHashes; i++ {
		elementsToHash = elementsToHash[:0]
		elementsToHash = append(elementsToHash, tmpHash[:]...)

		endPos := bytesPointer + maxBytesToAdd*2
		if endPos > len(bytecode) {
			endPos = len(bytecode)
		}
		subsetBytecode := bytecode[bytesPointer:endPos]
		bytesPointer += maxBytesToAdd * 2

		// 优化的十六进制解析
		tmpElem := uint64(0)
		counter := 0
		shift := uint64(0)

		for j := 0; j < maxBytesToAdd; j++ {
			var byteVal uint64
			if j*2+1 < len(subsetBytecode) {
				b1 := hexCharToValue(subsetBytecode[j*2])
				b2 := hexCharToValue(subsetBytecode[j*2+1])
				byteVal = uint64(b1<<4 | b2)
			}

			tmpElem |= byteVal << shift
			shift += 8
			counter++

			if counter == BYTECODE_BYTES_ELEMENT {
				elementsToHash = append(elementsToHash, tmpElem)
				tmpElem = 0
				shift = 0
				counter = 0
			}
		}

		copy(in[:], elementsToHash[4:12])
		copy(capacity[:], elementsToHash[:4])

		tmpHash = Hash(in, capacity)
	}

	return ArrayToScalar(tmpHash[:])
}

// 优化版本3：添加缓存机制的最终版本
func HashContractBytecodeBigIntOptimized3(bc string) *big.Int {
	// 检查缓存
	if cached, ok := hashCache.Load(bc); ok {
		return cached.(*big.Int)
	}

	// 计算哈希
	result := HashContractBytecodeBigIntOptimized(bc)

	// 添加到缓存（如果缓存未满）
	if atomic.LoadInt64(&cacheSize) < int64(maxCacheSize) {
		// 创建副本避免外部修改
		cachedResult := new(big.Int).Set(result)
		if _, loaded := hashCache.LoadOrStore(bc, cachedResult); !loaded {
			atomic.AddInt64(&cacheSize, 1)
		}
	}

	return result
}

// 清理缓存的函数
func ClearHashCache() {
	hashCache.Range(func(key, value interface{}) bool {
		hashCache.Delete(key)
		return true
	})
	atomic.StoreInt64(&cacheSize, 0)
}

// 获取缓存统计信息
func GetHashCacheStats() (size int64, maxSize int) {
	return atomic.LoadInt64(&cacheSize), maxCacheSize
}
