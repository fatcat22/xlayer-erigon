package stages

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ledgerwatch/log/v3"
)

func (r *DatastreamClientRunner) StartReadForXLayer(
	errorChan chan struct{},
	highestDSL2Block uint64,
	blockRange uint64,
) error {
	if r.isReading.Load() {
		return fmt.Errorf("tried starting datastream client runner thread while another is running")
	}

	r.stopRunner.Store(false)

	go func() {
		routineId := rand.Intn(1000000)

		log.Info(fmt.Sprintf("[%s] Started range downloading L2Blocks routine ID: %d", r.logPrefix, routineId))
		defer log.Info(fmt.Sprintf("[%s] Ended range downloading L2Blocks routine ID: %d", r.logPrefix, routineId))

		r.isReading.Store(true)
		defer r.isReading.Store(false)

		if err := r.dsClient.ReadEntriesToChannelXLayer(highestDSL2Block, blockRange); err != nil {
			time.Sleep(1 * time.Second)
			errorChan <- struct{}{}
			log.Warn(fmt.Sprintf("[%s] Error range downloading blocks from datastream", r.logPrefix), "error", err)
		}
	}()

	return nil
}

func (c *TestDatastreamClient) ReadEntriesToChannelXLayer(highestDSL2Block uint64, blockRange uint64) error {
	return nil
}
