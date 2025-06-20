package stagedsync

import (
	"context"
	"fmt"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/log/v3"
)

// VerificationCheckItem represents a block item that needs verification
type VerificationCheckItem struct {
	BlockHeight uint64    // Block height
	CheckTime   time.Time // Time when verification status should be checked
}

// GetVerificationCheckItems retrieves a list of blocks that need verification
// It gets the latest block height and VerifiedBlockHeight from the database
// Reads creation time for all heights from low to high, and adds VerificationCheckDelay
// Returns a slice containing block height and check time structures
func GetVerificationCheckItems(
	ctx context.Context,
	tx kv.Tx,
	verificationCheckDelay time.Duration,
	logger log.Logger,
) ([]VerificationCheckItem, error) {
	// 1. Get the latest block height from database
	currentHeader := rawdb.ReadCurrentHeader(tx)
	if currentHeader == nil {
		return nil, nil
	}
	currentBlockHeight := currentHeader.Number.Uint64()

	// 2. Get the VerifiedBlockHeight from database
	verifiedBlockHeight, err := stages.GetStageProgress(tx, stages.VerifiedBlockHeight)
	if err != nil {
		logger.Error("Failed to get VerifiedBlockHeight progress", "err", err)
		return nil, err
	}

	// 3. Ensure VerifiedBlockHeight <= current block height, otherwise panic
	if verifiedBlockHeight > currentBlockHeight {
		panic(fmt.Sprintf("VerifiedBlockHeight(%d) is greater than current block height(%d)",
			verifiedBlockHeight,
			currentBlockHeight))
	}

	// If no blocks need verification, return empty slice
	if verifiedBlockHeight == currentBlockHeight {
		return []VerificationCheckItem{}, nil
	}

	// 4. Read creation time for all heights from low to high, and add VerificationCheckDelay
	var verificationItems []VerificationCheckItem

	for blockHeight := verifiedBlockHeight + 1; blockHeight <= currentBlockHeight; blockHeight++ {
		// Read block header
		header := rawdb.ReadHeaderByNumber(tx, blockHeight)
		if header == nil {
			logger.Warn("Failed to read header for block", "blockHeight", blockHeight)
			continue
		}

		// Calculate check time: block creation time + VerificationCheckDelay
		blockTime := time.Unix(int64(header.Time), 0)
		checkTime := blockTime.Add(verificationCheckDelay)

		// Create verification check item
		item := VerificationCheckItem{
			BlockHeight: blockHeight,
			CheckTime:   checkTime,
		}

		verificationItems = append(verificationItems, item)
	}

	logger.Debug("Generated verification check items",
		"verifiedBlockHeight", verifiedBlockHeight,
		"currentBlockHeight", currentBlockHeight,
		"itemsCount", len(verificationItems))

	return verificationItems, nil
}
