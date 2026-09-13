package txn

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/nathanaday/claude-atlas/internal/vault"
)

// lock takes the vault's advisory lock so two sessions cannot apply at once. It waits a
// few seconds for a busy vault, then fails closed.
func lock(v *vault.Vault) (func(), error) {
	if err := os.MkdirAll(v.Path(vault.MetaDir), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(v.Path(vault.MetaDir+"/lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				f.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK || time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("the vault is locked by another atlas operation (%s)", v.Root)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
