//go:build !linux

package server

// readLinuxDiskUsage 读取并返回相关数据。
func readLinuxDiskUsage(root string) (int64, int64) {
	return 0, 0
}
