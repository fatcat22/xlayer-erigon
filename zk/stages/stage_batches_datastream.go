package stages

import (
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"

	"time"

	"github.com/ledgerwatch/log/v3"
)

type DatastreamClientRunner struct {
	dsClient   DatastreamClient
	logPrefix  string
	stopRunner atomic.Bool
	isReading  atomic.Bool
}

func NewDatastreamClientRunner(dsClient DatastreamClient, logPrefix string) *DatastreamClientRunner {
	return &DatastreamClientRunner{
		dsClient:  dsClient,
		logPrefix: logPrefix,
	}
}

func (r *DatastreamClientRunner) StartRead(errorChan chan struct{}) error {
	if r.isReading.Load() {
		return fmt.Errorf("tried starting datastream client runner thread while another is running")
	}

	r.stopRunner.Store(false)

	go func() {
		routineId := rand.Intn(1000000)

		log.Info(fmt.Sprintf("[%s] Started downloading L2Blocks routine ID: %d", r.logPrefix, routineId))
		defer log.Info(fmt.Sprintf("[%s] Ended downloading L2Blocks routine ID: %d", r.logPrefix, routineId))

		r.isReading.Store(true)
		defer r.isReading.Store(false)

		if err := r.dsClient.ReadAllEntriesToChannel(); err != nil {
			time.Sleep(1 * time.Second)
			errorChan <- struct{}{}
			log.Warn(fmt.Sprintf("[%s] Error downloading blocks from datastream", r.logPrefix), "error", err)
		}
	}()

	return nil
}

func (r *DatastreamClientRunner) StartRangeRead(
	errorChan chan struct{},
	highestDSL2Block uint64,
	blockRange uint64,
) error {
	if r.isReading.Load() {
		return fmt.Errorf("tried starting datastream client runner thread while another is running")
	}

	r.stopRunner.Store(false)

	entryChan := r.dsClient.GetEntryChan()

	go func() {
		routineId := rand.Intn(1000000)

		log.Info(fmt.Sprintf("[%s] Started range downloading L2Blocks routine ID: %d", r.logPrefix, routineId))
		defer log.Info(fmt.Sprintf("[%s] Ended range downloading L2Blocks routine ID: %d", r.logPrefix, routineId))

		r.isReading.Store(true)
		defer r.isReading.Store(false)

		// Reconnect and send
		// if err := r.dsClient.HandleStart(); err != nil {
		// 	errorChan <- struct{}{}
		// 	log.Warn(fmt.Sprintf("[%s] Error on handle start datastream connection", r.logPrefix), "error", err)
		// 	return
		// }

		// first load up the header of the stream
		if _, err := r.dsClient.GetHeader(); err != nil {
			if strings.Contains(err.Error(), "broken pipe") {
				// reconnenct to ds if the last error is casued by broken pipe
				if err := r.dsClient.HandleStart(); err != nil {
					errorChan <- struct{}{}
					log.Warn(fmt.Sprintf("[%s] Error on retry handle start datastream connection", r.logPrefix), "error", err)
					return
				}
				// get header from ds again
				if _, err := r.dsClient.GetHeader(); err != nil {
					errorChan <- struct{}{}
					log.Warn(fmt.Sprintf("[%s] Error on retring getting block header from datastream", r.logPrefix), "error", err)
					return
				}
			} else {
				errorChan <- struct{}{}
				log.Warn(fmt.Sprintf("[%s] Error getting block header from datastream", r.logPrefix), "error", err)
				return
			}
		}

		lastFrom := uint64(0)
		lastTo := uint64(0)
		errorFlag := false
		progress := r.dsClient.GetProgressAtomic()
		for !r.stopRunner.Load() {
			// Wait until all entries in entryChan is consumed
			for len(*entryChan) > 0 {
				time.Sleep(100 * time.Millisecond)
				if r.stopRunner.Load() {
					return
				}
			}

			// Check for conn health
			if err := r.dsClient.HandleStart(); err != nil {
				time.Sleep(1 * time.Second)
				errorChan <- struct{}{}
				log.Warn(fmt.Sprintf("[%s] Error on handle start datastream connection", r.logPrefix), "error", err)
				return
			}

			from := progress.Load()
			log.Info("StartRangeRead", "from", from, "highestDSL2Block", highestDSL2Block)
			if from >= highestDSL2Block {
				break
			}

			if lastFrom == from && from > 0 {
				log.Info("StartRangeRead from equals to lastFrom", "from", from, "lastFrom", lastFrom)
				time.Sleep(1 * time.Second)
				continue
			}

			to := min(from+blockRange, highestDSL2Block)
			if lastTo == to {
				log.Info("StartRangeRead to equals to lastTo", "to", to, "lastTo", lastTo)
				break
			}

			log.Info("ReadRangeEntriesToChannel", "to", to)
			if err := r.dsClient.ReadRangeEntriesToChannel(to); err != nil {
				if !errorFlag {
					// Try to reconnect and get range again
					errorFlag = true
					continue
				} else {
					time.Sleep(1 * time.Second)
					errorChan <- struct{}{}
					log.Warn(fmt.Sprintf("[%s] Error downloading blocks from datastream", r.logPrefix), "error", err)
					return
				}
			} else {
				errorFlag = false
			}
			lastFrom = from
			lastTo = to
		}

		// Send stop signal
		if err := r.dsClient.TrySendStopSignal(); err != nil {
			time.Sleep(1 * time.Second)
			errorChan <- struct{}{}
			log.Warn(fmt.Sprintf("[%s] Error sending stop signal", r.logPrefix), "error", err)
			return
		}
	}()

	return nil
}

func (r *DatastreamClientRunner) StopRead() {
	r.stopRunner.Swap(true)
	r.dsClient.StopReadingToChannel()
}
