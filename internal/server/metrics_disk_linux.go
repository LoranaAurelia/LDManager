//go:build linux

package server

import "syscall"

// readLinuxDiskUsage 读取并返回相关数据。
func readLinuxDiskUsage(root string) (int64, int64) {
	if root == "" {
		root = "/"
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		return 0, 0
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return total, free
}
