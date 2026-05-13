package archive

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func ReadIDs(path string) (map[string]struct{}, error) {
	ids := make(map[string]struct{})
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return ids, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		ids[line] = struct{}{}
	}
	return ids, scanner.Err()
}

func Diff(before, after map[string]struct{}) []string {
	var added []string
	for id := range after {
		if _, ok := before[id]; !ok {
			added = append(added, id)
		}
	}
	return added
}

func EnsureFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}
