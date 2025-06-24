package stagedsync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/zk/hermez_db"
	"github.com/ledgerwatch/erigon/zk/nacos"
	"github.com/ledgerwatch/log/v3"
)

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

// isBlockVerifiedByAnalysisGroup checks if a block is verified by calling the analysis group API
func isBlockVerifiedByAnalysisGroup(
	ctx context.Context,
	blockHeight uint64,
	nacosClient *nacos.XlayerNacosClient,
	apiPath string,
	logger log.Logger,
) (bool, error) {
	// Prepare request payload
	request := AnalysisGroupAPIRequest{
		Height: blockHeight,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		logger.Error("Failed to marshal request body", "blockHeight", blockHeight, "err", err)
		return false, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Set headers
	reqHeaders := map[string]string{
		"Content-Type": "application/json",
	}

	// Make the request
	respBody, err := nacosClient.Post(apiPath, requestBody, reqHeaders)
	if err != nil {
		logger.Error("Failed to call analysis group API", "blockHeight", blockHeight, "apiPath", apiPath, "err", err)
		return false, fmt.Errorf("failed to call analysis group API: %w", err)
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
			"apiPath", apiPath,
			"code", apiResponse.Code,
			"msg", apiResponse.Msg)
		return false, fmt.Errorf("analysis group API error: code=%s, msg=%s", apiResponse.Code, apiResponse.Msg)
	}

	// Check verification result
	isVerified := apiResponse.Data.ValidResult == "true"
	logger.Debug("Analysis group API response",
		"blockHeight", blockHeight,
		"apiPath", apiPath,
		"validResult", apiResponse.Data.ValidResult,
		"isVerified", isVerified)

	return isVerified, nil
}

// SpawnAnalysisGroupVerificationCheckStage processes verification using batch delay logic
func SpawnAnalysisGroupVerificationCheckStage(
	ctx context.Context,
	s *StageState,
	db kv.RwDB,
	verificationConfig ethconfig.AnalysisGroupVerificationConfig,
	logger log.Logger,
) error {
	// Get current highest batch number
	var latestBatchNo uint64
	var err error
	err = db.View(ctx, func(tx kv.Tx) error {
		latestBatchNo, err = stages.GetStageProgress(tx, stages.HighestSeenBatchNumber)
		return err
	})
	if err != nil {
		logger.Error("Failed to get latest batch number", "err", err)
		return err
	}

	// Calculate target batch number for verification: latest batch - batch delay
	if latestBatchNo < verificationConfig.BatchDelay {
		logger.Info("Latest batch number is less than batch delay, skipping analysis group verification check stage",
			"latestBatch", latestBatchNo,
			"batchDelay", verificationConfig.BatchDelay)
		return nil
	}
	targetBatchNumber := latestBatchNo - verificationConfig.BatchDelay

	logger.Debug("Calculated target batch for verification",
		"latestBatch", latestBatchNo,
		"batchDelay", verificationConfig.BatchDelay,
		"targetBatch", targetBatchNumber)

	// Get the highest block number in the target batch and current verified block height
	var currentVerifiedBlockHeight uint64
	var highestBlockInTargetBatch uint64
	var foundBlockInTargetBatch bool
	var foundCurrentVerifiedBlockHeight bool
	err = db.View(ctx, func(tx kv.Tx) error {
		hermezDb := hermez_db.NewHermezDbReader(tx)
		highestBlockInTargetBatch, foundBlockInTargetBatch, err = hermezDb.GetHighestBlockInBatch(targetBatchNumber)
		if err != nil {
			logger.Error("Failed to get highest block in target batch", "targetBatch", targetBatchNumber, "err", err)
			return err
		}
		currentVerifiedBlockHeight, err = stages.GetStageProgress(tx, stages.AnalysisGroupVerifiedBlockHeight)
		if err == nil {
			foundCurrentVerifiedBlockHeight = true
		}
		return nil
	})
	if err != nil {
		logger.Error("Failed to get highest block in target batch", "targetBatch", targetBatchNumber, "err", err)
		return err
	}
	if !foundBlockInTargetBatch || highestBlockInTargetBatch == 0 {
		logger.Info("No blocks found in target batch", "targetBatch", targetBatchNumber)
		return nil
	}

	logger.Debug("Found highest block in target batch",
		"targetBatch", targetBatchNumber,
		"highestBlock", highestBlockInTargetBatch)

	// Only verify if the target block is higher than current verified block
	if foundCurrentVerifiedBlockHeight && highestBlockInTargetBatch <= currentVerifiedBlockHeight {
		logger.Debug("Target block is not higher than current verified block",
			"targetBlock", highestBlockInTargetBatch,
			"currentVerified", currentVerifiedBlockHeight)
		return nil
	}

	// Call analysis group API to verify the block
	var isVerified bool
	if verificationConfig.SkipAPI {
		// Skip API call and directly mark as verified
		isVerified = true
		logger.Info("Skipping analysis group API call, marking block as verified",
			"blockHeight", highestBlockInTargetBatch,
			"targetBatch", targetBatchNumber)
	} else {
		// Call analysis group API to check verification
		isVerified, err = isBlockVerifiedByAnalysisGroup(ctx, highestBlockInTargetBatch, verificationConfig.NacosClient, verificationConfig.APIPath, logger)
		if err != nil {
			logger.Error("Failed to check block verification",
				"blockHeight", highestBlockInTargetBatch,
				"err", err)
			return err
		}
	}

	// Update verified block height in database if verification successful
	if isVerified {
		err = db.Update(ctx, func(tx kv.RwTx) error {
			err = stages.SaveStageProgress(tx, stages.AnalysisGroupVerifiedBlockHeight, highestBlockInTargetBatch)
			return err
		})
		if err != nil {
			logger.Error("Failed to save verified block height",
				"blockHeight", highestBlockInTargetBatch,
				"err", err)
			return err
		}

		logger.Info("Successfully verified and updated block height",
			"blockHeight", highestBlockInTargetBatch,
			"targetBatch", targetBatchNumber,
			"skipAPI", verificationConfig.SkipAPI)
	} else {
		logger.Debug("Block verification failed",
			"blockHeight", highestBlockInTargetBatch,
			"targetBatch", targetBatchNumber)
	}

	return nil
}
