package pay

import (
	"time"

	"github.com/go-co-op/gocron/v2"
)

func (p *Pay) runJobs() {
	var err error
	p.scheduler, err = gocron.NewScheduler()
	if err != nil {
		panic(err)
	}

	p.scheduler.NewJob(
		gocron.DurationJob(1*time.Hour),
		gocron.NewTask(p.settleAll),
	)
	p.scheduler.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(1, 1, 1))),
		gocron.NewTask(p.residencyAll),
	)
	p.scheduler.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(0, 0, 0))),
		gocron.NewTask(p.db.ResetDailyUsage),
	)

	p.scheduler.Start()
}

func (p *Pay) residencyAll() {
	// todo
}
