package chainkit

func (c *Chainkit) runJobs() {
	if c.scheduler == nil {
		return
	}
	log.Debug("chainkit runJobs, schedule check")
	c.scheduler.Every(5).Minute().SingletonMode().Do(c.check)
	// c.scheduler.Every(20).Seconds().SingletonMode().Do(c.check)
}
