package chainkit

import (
	"time"

	"github.com/go-co-op/gocron/v2"
)

func (c *Chainkit) runJobs() {
	if c.scheduler == nil {
		return
	}
	log.Debug("chainkit runJobs, schedule check")

	// Create job with v2 API
	_, err := c.scheduler.NewJob(
		// gocron.DurationJob(40*time.Second),
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(c.check),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		log.Error("failed to create check job", "error", err)
	}
}
