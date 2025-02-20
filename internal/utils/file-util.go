package utils

import (
	"fmt"
	"os"
	"time"
)

func GetLatestFile(files []os.FileInfo) (os.FileInfo, error) {
	var latestFile os.FileInfo
	var latestModTime time.Time

	for _, file := range files {
		if !file.IsDir() && (file == nil || file.ModTime().After(latestModTime)) {
			latestFile = file
			latestModTime = file.ModTime()
		}
	}

	if latestFile == nil {
		return nil, fmt.Errorf("empty file")
	}

	return latestFile, nil
}
