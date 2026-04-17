package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var validModes = map[string]struct{}{
	"rule":   {},
	"global": {},
	"direct": {},
}

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

func WriteSystemProxyEnabled(path string, enabled bool) error {
	return updateSystemProxyEnabled(path, boolString(enabled))
}

func ReadAllowLanEnabled(path string) (bool, error) {
	return readTopLevelBoolKey(path, "allow-lan", false)
}

func WriteAllowLanEnabled(path string, enabled bool) error {
	return updateTopLevelKey(path, "allow-lan", boolString(enabled))
}

func ReadTunEnabled(path string) (bool, error) {
	return readNestedBoolKey(path, "tun", "enable", false)
}

func WriteTunEnabled(path string, enabled bool) error {
	return updateNestedBoolKey(path, "tun", "enable", boolString(enabled))
}

func WriteMode(path, mode string) error {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return fmt.Errorf("mode is empty")
	}
	if _, ok := validModes[normalized]; !ok {
		return fmt.Errorf("unsupported mode %q", mode)
	}
	return updateTopLevelKey(path, "mode", normalized)
}

func updateSystemProxyEnabled(path, value string) error {
	return updateNestedBoolKey(path, "system-proxy", "enable", value)
}

func updateTopLevelKey(path, key, value string) error {
	lines, err := readConfigLines(path)
	if err != nil {
		return err
	}

	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if leadingWhitespace(rawLine) == 0 && strings.HasPrefix(line, key+":") {
			lines[i] = key + ": " + value
			return writeConfigLines(path, lines)
		}
	}

	lines = append(lines, key+": "+value)
	return writeConfigLines(path, lines)
}

func readTopLevelBoolKey(path, key string, defaultValue bool) (bool, error) {
	lines, err := readConfigLines(path)
	if err != nil {
		return false, err
	}
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if leadingWhitespace(rawLine) == 0 && strings.HasPrefix(line, key+":") {
			return parseBoolValue(strings.TrimSpace(strings.TrimPrefix(line, key+":"))), nil
		}
	}
	return defaultValue, nil
}

func readNestedBoolKey(path, parent, child string, defaultValue bool) (bool, error) {
	lines, err := readConfigLines(path)
	if err != nil {
		return false, err
	}

	inlinePrefix := parent + "." + child + ":"
	blockPrefix := parent + ":"
	childPrefix := child + ":"

	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, inlinePrefix) {
			return parseBoolValue(strings.TrimSpace(strings.TrimPrefix(line, inlinePrefix))), nil
		}
		if !strings.HasPrefix(line, blockPrefix) {
			continue
		}

		parentIndent := leadingWhitespace(rawLine)
		for j := i + 1; j < len(lines); j++ {
			childRaw := lines[j]
			childLine := strings.TrimSpace(childRaw)
			if childLine == "" || strings.HasPrefix(childLine, "#") {
				continue
			}
			childIndent := leadingWhitespace(childRaw)
			if childIndent <= parentIndent {
				break
			}
			if strings.HasPrefix(childLine, childPrefix) {
				return parseBoolValue(strings.TrimSpace(strings.TrimPrefix(childLine, childPrefix))), nil
			}
		}
	}

	return defaultValue, nil
}

func updateNestedBoolKey(path, parent, child, value string) error {
	lines, err := readConfigLines(path)
	if err != nil {
		return err
	}

	inlinePrefix := parent + "." + child + ":"
	blockPrefix := parent + ":"
	childPrefix := child + ":"

	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, inlinePrefix) {
			lines[i] = leadingIndent(rawLine) + inlinePrefix + " " + value
			return writeConfigLines(path, lines)
		}
		if !strings.HasPrefix(line, blockPrefix) {
			continue
		}

		parentIndent := leadingWhitespace(rawLine)
		insertAt := i + 1
		for j := i + 1; j < len(lines); j++ {
			childRaw := lines[j]
			childLine := strings.TrimSpace(childRaw)
			if childLine == "" || strings.HasPrefix(childLine, "#") {
				continue
			}
			childIndent := leadingWhitespace(childRaw)
			if childIndent <= parentIndent {
				break
			}
			if strings.HasPrefix(childLine, childPrefix) {
				lines[j] = leadingIndent(childRaw) + childPrefix + " " + value
				return writeConfigLines(path, lines)
			}
			insertAt = j + 1
		}

		lines = insertLine(lines, insertAt, leadingIndent(rawLine)+"  "+childPrefix+" "+value)
		return writeConfigLines(path, lines)
	}

	lines = append(lines, parent+":", "  "+childPrefix+" "+value)
	return writeConfigLines(path, lines)
}

func readConfigLines(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("mixin config path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return []string{}, nil
	}
	return strings.Split(content, "\n"), nil
}

func writeConfigLines(path string, lines []string) error {
	if path == "" {
		return fmt.Errorf("mixin config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func parseBoolValue(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func insertLine(lines []string, index int, line string) []string {
	if index < 0 {
		index = 0
	}
	if index >= len(lines) {
		return append(lines, line)
	}

	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = line
	return lines
}

func leadingIndent(line string) string {
	return line[:leadingWhitespace(line)]
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
