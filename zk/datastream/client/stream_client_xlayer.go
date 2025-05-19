package client

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ledgerwatch/erigon/zk/datastream/proto/github.com/0xPolygonHermez/zkevm-node/state/datastream"
	"github.com/ledgerwatch/erigon/zk/datastream/types"
	"github.com/ledgerwatch/log/v3"
)

func (c *StreamClient) ReadEntriesToChannelXLayer(highestDSL2Block uint64, blockRange uint64) (err error) {
	defer func() {
		if err != nil {
			c.lastError = err
		}
	}()
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("context done - stopping")
	default:
	}

	// first load up the header of the stream
	if _, err = c.GetHeader(); err != nil {
		err = fmt.Errorf("GetHeader: %w", err)
		return err
	}

	errorFlag := false
	progress := c.GetProgressAtomic()
	for {
		// Wait until all entries in the entry channel is consumed
		for len(c.entryChan) > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		// Reset any client errors
		if err = c.HandleStart(); err != nil {
			err = fmt.Errorf("HandleStart: %w", err)
			return err
		}

		from := progress.Load()
		if from >= highestDSL2Block {
			break
		}

		to := min(from+blockRange, highestDSL2Block)
		if err = c.readRangeEntriesToChannel(to); err != nil {
			// Check for ds server inactivity timeout
			if !errorFlag {
				errorFlag = true
				continue
			}

			return fmt.Errorf("readRangeEntriesToChannel: %w", err)
		}

		errorFlag = false
	}

	// Send stop signal
	if err = c.trySendStopSignal(); err != nil {
		return err
	}

	return nil
}

// Get all entries from the DS server from the current stage progress until to block number,
// and sends the data into FullL2Blocks with transactions into the channel.
func (c *StreamClient) readRangeEntriesToChannel(to uint64) error {
	c.stopReadingToChannel.Store(false)

	// Send start command
	toEntry, err := c.initiateRangeDownload(to)
	if err != nil {
		return err
	}

	err = c.readRangeFullL2BlocksToChannel(toEntry)
	c.setStreaming(false)

	if err != nil {
		return fmt.Errorf("readRangeFullL2BlocksToChannel: %w", err)
	}

	return nil
}

func (c *StreamClient) initiateRangeDownload(to uint64) (uint64, error) {
	var start *types.BookmarkProto
	progress := c.progress.Load()
	if progress == 0 {
		start = types.NewBookmarkProto(0, datastream.BookmarkType_BOOKMARK_TYPE_BATCH)
	} else {
		start = types.NewBookmarkProto(progress+1, datastream.BookmarkType_BOOKMARK_TYPE_L2_BLOCK)
	}

	sb, err := start.Marshal()
	if err != nil {
		return 0, err
	}

	end := types.NewBookmarkProto(to, datastream.BookmarkType_BOOKMARK_TYPE_L2_BLOCK)
	eb, err := end.Marshal()
	if err != nil {
		return 0, err
	}

	if _, err := c.initiateRangeDownloadBookmark(sb, eb); err != nil {
		return 0, fmt.Errorf("initiateRangeDownloadBookmark: %w", err)
	}

	packet, err := c.readBuffer(8)
	if err != nil {
		return 0, err
	}
	return UnmarshalToEntryNumber(packet)
}

// Runs the prerequisites for range entries download
func (c *StreamClient) initiateRangeDownloadBookmark(startBookmark []byte, endBookmark []byte) (*types.ResultEntry, error) {
	if err := c.stopStreaming(); err != nil {
		return nil, fmt.Errorf("stopStreaming: %w", err)
	}

	// send CmdStartBookmark command
	if err := c.sendRangeBookmarkCmd(startBookmark, endBookmark); err != nil {
		return nil, fmt.Errorf("sendRangeBookmarkCmd: %w", err)
	}

	c.setStreaming(true)

	re, err := c.afterStartCommand()
	if err != nil {
		return re, fmt.Errorf("afterStartCommand: %w", err)
	}

	return re, nil
}

// Read all entries from the server until the end of the range toBlock.
// Parses the data into FullL2Blocks with transactions and sends them to a channel
func (c *StreamClient) readRangeFullL2BlocksToChannel(toEntry uint64) (err error) {
	readNewProto := true
	entryNum := uint64(0)
	parsedProto := interface{}(nil)
LOOP:
	for {
		select {
		default:
		case <-c.ctx.Done():
			return fmt.Errorf("context done - stopping")
		}

		if c.stopReadingToChannel.Load() {
			break LOOP
		}

		if err := c.resetReadTimeout(); err != nil {
			return err
		}

		if readNewProto {
			if parsedProto, entryNum, err = ReadParsedProto(c); err != nil {
				return err
			}
			readNewProto = false
		}
		c.lastWrittenTime.Store(time.Now().UnixNano())

		switch parsedProto := parsedProto.(type) {
		case *types.BookmarkProto:
			readNewProto = true
			continue
		case *types.BatchStart:
			c.currentFork = parsedProto.ForkId
		case *types.GerUpdate:
		case *types.BatchEnd:
		case *types.FullL2Block:
			parsedProto.ForkId = c.currentFork
			log.Trace("[Datastream client] writing block to channel", "blockNumber", parsedProto.L2BlockNumber, "batchNumber", parsedProto.BatchNumber)
		default:
			return fmt.Errorf("unexpected entry type: %v", parsedProto)
		}
		select {
		case c.entryChan <- parsedProto:
			readNewProto = true
		default:
			time.Sleep(10 * time.Microsecond)
		}

		// Reach the end of range. Do not send stop signal here as range read may continue
		if entryNum == toEntry || c.header.TotalEntries <= entryNum+1 {
			log.Trace("[Datastream client] reached the current end of the range read", "toEntry", toEntry, "entryNum", entryNum, "header_totalEntries", c.header.TotalEntries)
			break LOOP
		}
	}

	return nil
}

// sendRangeBookmarkCmd sends the CmdRangeBookmark with the start and end bookmark value.
func (c *StreamClient) sendRangeBookmarkCmd(startBookmark []byte, endBookmark []byte) error {
	command := CmdRangeBookmark
	// Send the command
	if err := c.sendCommand(command); err != nil {
		return err
	}

	// Send start bookmark
	if err := c.writeToConn(uint32(len(startBookmark))); err != nil {
		return err
	}
	if err := c.writeToConn(startBookmark); err != nil {
		return err
	}

	// Send end bookmark
	if err := c.writeToConn(uint32(len(endBookmark))); err != nil {
		return err
	}
	if err := c.writeToConn(endBookmark); err != nil {
		return err
	}

	return nil
}

func UnmarshalToEntryNumber(data []byte) (uint64, error) {
	return binary.BigEndian.Uint64(data[:8]), nil
}
