package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Exclusion rules
var excludedDirs = map[string]bool{
	"node_modules":    true,
	".next":           true,
	".nuxt":           true,
	"dist":            true,
	"build":           true,
	".cache":          true,
	"__pycache__":     true,
	".venv":           true,
	"venv":            true,
	"env":             true,
	".git":            true,
	"coverage":        true,
	".nyc_output":     true,
	".docker":         true,
	".pytest_cache":   true,
	".tox":            true,
	"vendor":          true,
	"chimera-outputs": true,
}

var excludedExtensions = map[string]bool{
	".log": true,
	".tmp": true,
}

// Generate generates a smart tree with exclusions
func Generate(rootDir string, maxLines int) (string, int, error) {
	var lines []string
	totalLines := 0

	err := walk(rootDir, rootDir, "", true, &lines, &totalLines, maxLines)
	if err != nil {
		return "", 0, err
	}

	tree := strings.Join(lines, "\n")
	return tree, totalLines, nil
}

func walk(root, dir, prefix string, isLast bool, lines *[]string, totalLines *int, maxLines int) error {
	if *totalLines >= maxLines {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Filter excluded
	var filtered []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if excludedDirs[name] {
			// Show excluded with count
			count := countItems(filepath.Join(dir, name))
			rel, _ := filepath.Rel(root, filepath.Join(dir, name))
			line := fmt.Sprintf("%s├── %s/ [excluded · ~%d items]", prefix, rel, count)
			*lines = append(*lines, line)
			*totalLines++
			continue
		}

		ext := filepath.Ext(name)
		if excludedExtensions[ext] {
			continue
		}

		filtered = append(filtered, entry)
	}

	for i, entry := range filtered {
		if *totalLines >= maxLines {
			break
		}

		isLastEntry := i == len(filtered)-1
		name := entry.Name()
		rel, _ := filepath.Rel(root, filepath.Join(dir, name))

		connector := "├──"
		if isLastEntry {
			connector = "└──"
		}

		if entry.IsDir() {
			line := fmt.Sprintf("%s%s %s/", prefix, connector, rel)
			*lines = append(*lines, line)
			*totalLines++

			newPrefix := prefix
			if isLastEntry {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}

			walk(root, filepath.Join(dir, name), newPrefix, isLastEntry, lines, totalLines, maxLines)
		} else {
			line := fmt.Sprintf("%s%s %s", prefix, connector, rel)
			*lines = append(*lines, line)
			*totalLines++
		}
	}

	return nil
}

func countItems(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		count++
		if count > 1000 {
			return filepath.SkipDir
		}
		return nil
	})
	return count
}
