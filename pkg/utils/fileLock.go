package utils

import (
	"flag"
	"os"

	// lijj32: syscall.Flock() does not support windows,
	// so use fslock package instead to support all platform.
	"github.com/juju/fslock"
)

const defaultLockFile = "/tmp/sriovlockfile"

var (
	lockFileName   = defaultLockFile
	globalFileLock = fslock.New(lockFileName)
)

func init() {
	flag.StringVar(&lockFileName, "lockFileName", defaultLockFile, "lock file name")

	_, err := os.Stat(lockFileName)
	if err != nil && os.IsNotExist(err) {
		_, _ = os.Create(lockFileName)
	}
}

func LockFile() error {
	return globalFileLock.Lock()
}

func UnlockFile() error {
	return globalFileLock.Unlock()
}
