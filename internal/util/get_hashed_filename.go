package util

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

func GetHashedFilename(content []byte, originalFileName string) string {
	hash := sha256.New()
	hash.Write(content)
	hashedSuffix := fmt.Sprintf("%x", hash.Sum(nil))[:12] // Short hash
	ext := filepath.Ext(originalFileName)
	outputFileName := fmt.Sprintf("%s_%s%s", strings.TrimSuffix(originalFileName, ext), hashedSuffix, ext)
	return outputFileName
}
