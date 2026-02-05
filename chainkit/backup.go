package chainkit

import "fmt"

func (r *Chainkit) backup() error {
	r.backupLockMu.Lock()
	if r.backupRunning {
		r.backupLockMu.Unlock()
		return fmt.Errorf("backup is running")
	}
	r.backupRunning = true
	r.backupLockMu.Unlock()

	go func() {
		defer func() {
			r.backupLockMu.Lock()
			r.backupRunning = false
			r.backupLockMu.Unlock()
		}()

		log.Info("begin backup all")
		pids, maxNonce, err := r.nodeDB.GetAllProcess()
		if err != nil {
			log.Error("backup get all process failed", "err", err)
			return
		}

		for i, pid := range pids {
			log.Info("backup process begin", "pid", pid, "index", i+1, "total", len(pids), "maxNonce", maxNonce[i])
			err = r.backupProcess(pid, maxNonce[i])
			if err != nil {
				log.Error("backup process failed", "pid", pid, "err", err)
				return
			}
		}
		log.Info("backup all process end")
	}()

	return nil
}

func (r *Chainkit) backupProcess(pid string, maxNonce int64) error {
	log.Info("backup process ", "pid", pid, "maxNonce", maxNonce)
	for nonce := int64(0); nonce <= maxNonce; nonce++ {
		log.Debug(fmt.Sprintf("==> %d/%d", nonce, maxNonce))
		msg, err := r.nodeDB.GetMessageByNonce(pid, nonce)
		if err != nil {
			log.Warn("backup get message by nonce failed", "pid", pid, "nonce", nonce, "err", err)
			continue
		}
		if msg == nil {
			continue
		}

		err = r.Upload(*msg)
		if err != nil {
			return err
		}
	}
	log.Info("backup process end", "pid", pid, "maxNonce", maxNonce)
	return nil
}
