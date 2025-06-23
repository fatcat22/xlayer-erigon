package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/node/nodecfg"
	"github.com/ledgerwatch/erigon/smt/pkg/blockinfo"
	"github.com/ledgerwatch/erigon/zk/nacos"
	"github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli/v2"
)

func ApplyFlagsForEthXLayerConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	sequencerBlockSealTime := cfg.Zk.SequencerBlockSealTime
	sequencerBatchSealTime := cfg.Zk.SequencerBatchSealTime
	sequencerMaxBlockSealTimeVal := ctx.String(utils.SequencerMaxBlockSealTime.Name)
	sequencerMaxBlockSealTime, err := time.ParseDuration(sequencerMaxBlockSealTimeVal)
	if err != nil || sequencerBlockSealTime > sequencerMaxBlockSealTime || sequencerMaxBlockSealTime > sequencerBatchSealTime {
		panic(fmt.Sprintf("Got error: %v, sequencer-block-seal-time: %s, sequencer-max-block-seal-time: %s, sequencer-batch-seal-time: %s", err, sequencerBlockSealTime, sequencerMaxBlockSealTime, sequencerBatchSealTime))
	}

	sequencerBatchCounterPercentage := ctx.Int(utils.SequencerBatchCounterPercentage.Name)
	err = vm.SetBatchCounterLimitPercentage(sequencerBatchCounterPercentage)
	if err != nil {
		panic(fmt.Sprintf("Got error: %v, sequencer-batch-counter-percentage: %d", err, sequencerBatchCounterPercentage))
	}

	cfg.XLayer = ethconfig.XLayerConfig{
		Apollo: ethconfig.ApolloClientConfig{
			Enable: ctx.Bool(utils.ApolloEnableFlag.Name),
			IP:     ctx.String(utils.ApolloIPAddr.Name),
			AppID:  ctx.String(utils.ApolloAppId.Name),
		},
		Nacos: ethconfig.NacosConfig{
			URLs:               ctx.String(utils.NacosURLsFlag.Name),
			NamespaceId:        ctx.String(utils.NacosNamespaceIdFlag.Name),
			ApplicationName:    ctx.String(utils.NacosApplicationNameFlag.Name),
			ExternalListenAddr: ctx.String(utils.NacosExternalListenAddrFlag.Name),
		},
		EnableInnerTx:                     ctx.Bool(utils.AllowInternalTransactions.Name),
		SequencerBatchSleepDuration:       ctx.Duration(utils.SequencerBatchSleepDuration.Name),
		SequencerReplay:                   ctx.Bool(utils.SequencerReplay.Name),
		SequencerReplayHaltOnBatchNumber:  ctx.Uint64(utils.SequencerReplayHaltOnBatchNumber.Name),
		SequencerReplayExternalDatastream: ctx.Bool(utils.SequencerReplayExternalDatastream.Name),
		SequencerReplayL1SyncOnly:         ctx.Bool(utils.SequencerReplayL1SyncOnly.Name),
		StandaloneSMTDatabase:             ctx.Bool(utils.StandaloneSMTDatabase.Name),
		BlockInfoConcurrent:               ctx.Bool(utils.BlockInfoConcurrent.Name),
		EnableAsyncCommit:                 ctx.Bool(utils.EnableAsyncCommit.Name),
		BulkAddTxs:                        ctx.Bool(utils.BulkAddTxsFlag.Name),
		BulkAddTxsSize:                    ctx.Int(utils.BulkAddTxsSizeFlag.Name),
		BulkAddTxsWaitTime:                ctx.Duration(utils.BulkAddTxsWaitTimeFlag.Name),
		EnableAddTxNotify:                 ctx.Bool(utils.EnableAddTxNotify.Name),
		SequencerSkipEmptyBlocks:          ctx.Bool(utils.SequencerSkipEmptyBlocks.Name),
		SequencerMaxBlockSealTime:         sequencerMaxBlockSealTime,
		GetLogsTimeout:                    ctx.Duration(utils.GetLogsTimeout.Name),
		GetLogsRetries:                    ctx.Int(utils.GetLogsRetries.Name),

		TraceLogPath:   ctx.String(utils.TraceLogPath.Name),
		EnableTraceLog: ctx.Bool(utils.EnableTraceLog.Name),
	}
	if cfg.XLayer.BlockInfoConcurrent {
		blockinfo.SetUseBlockInfoTree(true)
	}
	SetVerificationConfigs(ctx, cfg)

	// For X Layer, pre run
	utils.SetPreRunList(ctx, cfg)

	if ctx.IsSet(utils.ApolloNamespaceName.Name) {
		ns := strings.Split(ctx.String(utils.ApolloNamespaceName.Name), ",")
		for idx, item := range ns {
			ns[idx] = strings.TrimSpace(item)
		}
		cfg.XLayer.Apollo.NamespaceName = strings.Join(ns, ",")
	}
}

func ApplyFlagsForNodeXLayerConfig(ctx *cli.Context, cfg *nodecfg.Config) {
	cfg.Http.HttpApiKeys = ctx.String(utils.HTTPApiKeysFlag.Name)
	cfg.Http.MethodRateLimit = ctx.String(utils.MethodRateLimitFlag.Name)
}

func SetVerificationConfigs(ctx *cli.Context, cfg *ethconfig.Config) {
	verificationCheckDelayVal := ctx.String(utils.VerificationCheckDelay.Name)
	verificationCheckDelay, err := time.ParseDuration(verificationCheckDelayVal)
	if err != nil {
		panic(fmt.Sprintf("could not parse '%s': '%s'", utils.VerificationCheckDelay.Name, verificationCheckDelayVal))
	}
	cfg.XLayer.AnalysisGroupVerification.CheckDelay = verificationCheckDelay

	cfg.XLayer.AnalysisGroupVerification.SkipAPI = ctx.Bool(utils.SkipAnalysisGroupAPI.Name)
	cfg.XLayer.AnalysisGroupVerification.APIPath = ctx.String(utils.AnalysisGroupAPIPath.Name)
	log.Info("yangzhe: AnalysisGroupVerification", "delay", verificationCheckDelay, "skipAPI", cfg.XLayer.AnalysisGroupVerification.SkipAPI, "apiPath", cfg.XLayer.AnalysisGroupVerification.APIPath)

	// Set AnalysisGroupServiceName
	if ctx.IsSet(utils.AnalysisGroupServiceName.Name) {
		serviceName := ctx.String(utils.AnalysisGroupServiceName.Name)

		if cfg.XLayer.AnalysisGroupVerification.SkipAPI {
			log.Warn("skip analysis group api but service name is set", "service name", serviceName)
		}
		cfg.XLayer.AnalysisGroupVerification.NacosClient, err = nacos.NewNacosClient("", serviceName)
		if err != nil && !cfg.XLayer.AnalysisGroupVerification.SkipAPI {
			panic(fmt.Sprintf("failed to create nacos client for analysis group: %s", err))
		}
	}
}
