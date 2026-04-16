package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReadSystemProxyEnabled(path string) (bool, error) {
	if path == "" {
		return false, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inSystemProxy := false
	systemProxyIndent := -1

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, "system-proxy.enable:") {
			if strings.HasPrefix(line, "system-proxy:") {
				inSystemProxy = true
				systemProxyIndent = leadingWhitespace(rawLine)
				continue
			}

			if inSystemProxy {
				indent := leadingWhitespace(rawLine)
				if indent <= systemProxyIndent {
					inSystemProxy = false
					systemProxyIndent = -1
					continue
				}

				if strings.HasPrefix(line, "enable:") {
					value := strings.TrimSpace(strings.TrimPrefix(line, "enable:"))
					return strings.EqualFold(value, "true"), nil
				}
			}

			continue
		}

		value := strings.TrimSpace(strings.TrimPrefix(line, "system-proxy.enable:"))
		return strings.EqualFold(value, "true"), nil
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("scan %s: %w", path, err)
	}
	return false, nil
}

func leadingWhitespace(line string) int {
	count := 0
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			break
		}
		count++
	}
	return count
}
