package stagedsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// AnalysisGroupAPIResponse represents the response from analysis group API
type AnalysisGroupAPIResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ValidResult string `json:"validResult"`
	} `json:"data"`
}

// AnalysisGroupAPIRequest represents the request to analysis group API
type AnalysisGroupAPIRequest struct {
	Height uint64 `json:"height"`
}

// isBlockVerifiedByAnalysisGroup calls the analysis group API to check if a block is verified
// Returns true if the block is verified by analysis group, false otherwise
func isBlockVerifiedByAnalysisGroup(
	ctx context.Context,
	blockHeight uint64,
	apiBaseURL string,
	logger log.Logger,
) (bool, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Prepare request payload
	request := AnalysisGroupAPIRequest{
		Height: blockHeight,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		logger.Error("Failed to marshal request body", "blockHeight", blockHeight, "err", err)
		return false, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/v1/196/validHeight", apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		logger.Error("Failed to create HTTP request", "blockHeight", blockHeight, "url", url, "err", err)
		return false, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	logger.Debug("Calling analysis group API", "blockHeight", blockHeight, "url", url)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to call analysis group API", "blockHeight", blockHeight, "err", err)
		return false, fmt.Errorf("failed to call analysis group API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", "blockHeight", blockHeight, "err", err)
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		logger.Error("Analysis group API returned non-OK status",
			"blockHeight", blockHeight,
			"statusCode", resp.StatusCode,
			"response", string(respBody))
		return false, fmt.Errorf("analysis group API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var apiResponse AnalysisGroupAPIResponse
	if err := json.Unmarshal(respBody, &apiResponse); err != nil {
		logger.Error("Failed to unmarshal API response", "blockHeight", blockHeight, "response", string(respBody), "err", err)
		return false, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// Check if API call was successful
	if apiResponse.Code != "0" {
		logger.Error("Analysis group API returned error code",
			"blockHeight", blockHeight,
			"code", apiResponse.Code,
			"msg", apiResponse.Msg)
		return false, fmt.Errorf("analysis group API error: code=%s, msg=%s", apiResponse.Code, apiResponse.Msg)
	}

	// Check verification result
	isVerified := apiResponse.Data.ValidResult == "true"
	logger.Debug("Analysis group API response",
		"blockHeight", blockHeight,
		"validResult", apiResponse.Data.ValidResult,
		"isVerified", isVerified)

	return isVerified, nil
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

// ProcessVerificationChecks processes verification check items and updates the verified block height
// It finds the first block that passes verification from the analysis group API
// and updates the VerifiedBlockHeight in the database
func ProcessVerificationChecks(
	ctx context.Context,
	tx kv.RwTx,
	verificationItems []VerificationCheckItem,
	apiBaseURL string,
	logger log.Logger,
) error {
	// 1. Get current time and find items that are ready for verification
	currentTime := time.Now()
	var readyItems []VerificationCheckItem

	for _, item := range verificationItems {
		if !item.CheckTime.After(currentTime) {
			readyItems = append(readyItems, item)
		}
	}

	if len(readyItems) == 0 {
		logger.Debug("No verification items ready for checking", "currentTime", currentTime)
		return nil
	}

	logger.Debug("Found items ready for verification",
		"totalItems", len(verificationItems),
		"readyItems", len(readyItems),
		"currentTime", currentTime)

	// 2. Traverse ready items from back to front (highest to lowest block height)
	// Find the first block that passes verification
	var verifiedBlockHeight uint64
	foundVerifiedBlock := false

	for i := len(readyItems) - 1; i >= 0; i-- {
		item := readyItems[i]

		logger.Debug("Checking block verification",
			"blockHeight", item.BlockHeight,
			"checkTime", item.CheckTime)

		isVerified, err := isBlockVerifiedByAnalysisGroup(ctx, item.BlockHeight, apiBaseURL, logger)
		if err != nil {
			logger.Error("Failed to check block verification",
				"blockHeight", item.BlockHeight,
				"err", err)
			// Continue checking other blocks even if one fails
			continue
		}

		if isVerified {
			verifiedBlockHeight = item.BlockHeight
			foundVerifiedBlock = true
			logger.Info("Found verified block",
				"blockHeight", verifiedBlockHeight,
				"checkTime", item.CheckTime)
			break
		}

		logger.Debug("Block not verified yet",
			"blockHeight", item.BlockHeight)
	}

	// 3. Update VerifiedBlockHeight in database if a verified block was found
	if foundVerifiedBlock {
		err := stages.SaveStageProgress(tx, stages.VerifiedBlockHeight, verifiedBlockHeight)
		if err != nil {
			logger.Error("Failed to save VerifiedBlockHeight",
				"blockHeight", verifiedBlockHeight,
				"err", err)
			return fmt.Errorf("failed to save VerifiedBlockHeight: %w", err)
		}

		logger.Info("Updated VerifiedBlockHeight in database",
			"blockHeight", verifiedBlockHeight)
	} else {
		logger.Debug("No verified blocks found among ready items",
			"checkedItems", len(readyItems))
	}

	return nil
}
