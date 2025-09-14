package chainkit

func (c *Chainkit) runJobs() {
	c.scheduler.Every(5).Minute().SingletonMode().Do(c.check)
}
