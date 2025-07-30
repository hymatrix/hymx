package vmm

func (v *Vmm) runChanHandler() {
	v.wg.Add(1)
	defer v.wg.Done()
	for {
		select {
		case <-v.ctx.Done():

			return

		case meta := <-v.applyChan:

			if err := v.apply(meta); err != nil {
				log.Error("apply failed", "pid", meta.Pid, "itemId", meta.ItemId, "err", err)
			}

		case ckp := <-v.ckpChan:

			v.checkpoint(ckp.Pid, ckp.Res)

		}
	}
}
