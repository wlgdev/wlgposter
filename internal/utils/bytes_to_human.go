package utils

import "fmt"

func BytesToHuman(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div := int64(unit)
	exp := 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	value := float64(bytes) / float64(div)
	suffix := []string{"KB", "MB", "GB", "TB", "PB"}[exp]

	return fmt.Sprintf("%.2f%s", value, suffix)
}
