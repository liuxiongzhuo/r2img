//go:build windows

package utils

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func GetFileAccessTime(path string) (time.Time, error) {
	_, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}

	// Windows 获取文件访问时间
	pointer, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return time.Time{}, err
	}
	defer windows.CloseHandle(pointer)

	var fileInfo windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(pointer, &fileInfo)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, fileInfo.LastAccessTime.Nanoseconds()), nil

}
