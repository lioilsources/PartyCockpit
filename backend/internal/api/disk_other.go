//go:build !unix

package api

func freeBytes(string) int64 { return -1 }
