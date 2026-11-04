//go:build unix

package api

import "syscall"

// freeBytes returns the space available to us on dir's filesystem, -1 if unknown.
func freeBytes(dir string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
