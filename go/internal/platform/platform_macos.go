//go:build darwin

package platform

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

func GetFileAccessTime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	stat := fi.Sys().(*syscall.Stat_t)
	return time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec), nil
}

// FileInfo ... (与 windows 版本相同, 复用)
type FileInfo struct {
	Path  string
	Size  int64
	Atime time.Time
}

// CleanOldFiles ... (与 windows 版本相同逻辑，但现在属于 platform 包)
func CleanOldFiles(dir string, maxSize, freeSize int64) error {
	var files []FileInfo
	var totalSize int64 = 0

	// 遍历文件夹
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 忽略目录
		if info.IsDir() {
			return nil
		}

		atime, err := GetFileAccessTime(path)
		if err != nil {
			log.Printf("读取文件时期失败: %v\n", err)
			return nil // 忽略错误，继续清理其他文件
		}
		files = append(files, FileInfo{Path: path, Size: info.Size(), Atime: atime})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		log.Printf("遍历文件夹失败: %v\n", err)
		return err
	}

	// 如果总大小未超过限制，则不删除
	if totalSize < maxSize {
		return nil
	}

	// 按访问时间排序（最旧的在前）
	sort.Slice(files, func(i, j int) bool {
		return files[i].Atime.Before(files[j].Atime)
	})

	// 逐步删除最旧的文件，直到释放 FreeSize
	var deletedSize int64 = 0
	for _, file := range files {
		if deletedSize >= (totalSize - freeSize) {
			break
		}
		err := os.Remove(file.Path)
		if err != nil {
			log.Printf("删除文件失败: %s, 错误: %v\n", file.Path, err)
			continue
		}
		deletedSize += file.Size
		log.Printf("成功删除文件: %s\n", file.Path)
	}

	return nil
}
