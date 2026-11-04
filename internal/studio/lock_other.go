//go:build !unix

package studio

// processAlive conservatively assumes the lock holder is alive on platforms
// without a cheap liveness probe; remove the lock file manually if stale.
func processAlive(pid int) bool { return true }
