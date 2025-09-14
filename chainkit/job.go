package chainkit

func (c *Chainkit) runJobs() {
	if c.scheduler == nil {
		return
	}
	c.scheduler.Every(5).Minute().SingletonMode().Do(c.check)
}
