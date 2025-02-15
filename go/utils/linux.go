//go:build linux

package utils

import (
	"os"
	"time"
)

func GetFileAccessTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
