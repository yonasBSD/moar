package internal

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	log "github.com/sirupsen/logrus"
)

var searchHistory []string

const maxSearchHistoryEntries = 100
const moorSearchHistoryFileName = ".moor_search_history"

// If this returns nil it means there were problems and we shouldn't touch the
// history.
func loadSearchHistory() []string {
	history, err := loadMoorSearchHistory()
	if err != nil {
		log.Infof("Could not load moor search history: %v", err)
		// IO Error, give up
		return nil
	}
	if history != nil {
		log.Infof("Loaded %d search history entries from ~/%s", len(history), moorSearchHistoryFileName)
		return history
	}

	history, err = loadLessSearchHistory()
	if err != nil {
		log.Infof("Could not import less search history: %v", err)
		return nil
	}
	if history == nil {
		return nil
	}

	log.Infof("Imported %d search history entries from less", len(history))
	return history
}

// Try loading search history from ~/.moor_search_history.
// Returns (nil, nil) if the file doesn't exist. Returns history slice or error.
func loadMoorSearchHistory() ([]string, error) {
	lines := []string{}
	err := iterateFileByLines(moorSearchHistoryFileName, func(line string) {
		if len(line) > 640 {
			// Line too long, 640 chars should be enough for anyone
			return
		}

		lines = append(lines, line)
		if len(lines) > maxSearchHistoryEntries {
			// Throw away the first (oldest) history line, we don't want more
			// than this
			lines = lines[1:]
		}
	})
	if errors.Is(err, os.ErrNotExist) {
		// No history file found, not a problem but no history either, return
		// nil rather than empty slice
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return cleanSearchHistory(lines), nil
}

// Return a new string with any unprintable characters removed
func withoutUnprintables(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// File format ref: https://unix.stackexchange.com/a/246641/384864
func loadLessSearchHistory() ([]string, error) {
	fileNames := []string{".lesshst", "_lesshst"}
	for _, fileName := range fileNames {
		lines := []string{}
		err := iterateFileByLines(fileName, func(line string) {
			if !strings.HasPrefix(line, "\"") {
				// Not a search history line
				return
			}
			if len(line) > 640 {
				// Line too long, 640 chars should be enough for anyone
				return
			}

			lines = append(lines, withoutUnprintables(line[1:])) // Strip leading "
			if len(lines) > maxSearchHistoryEntries {
				// Throw away the first (oldest) history line, we don't want more
				// than this
				lines = lines[1:]
			}
		})

		if errors.Is(err, os.ErrNotExist) {
			// No such file, try next
			continue
		}

		if err != nil {
			return nil, err
		}

		return cleanSearchHistory(lines), nil
	}

	// No history files found, not a problem but no history either, return
	return nil, nil
}

func iterateFileByLines(filename string, processLine func(string)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get user home dir: %w", err)
	}

	path := filepath.Join(home, filename)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open %s: %w", path, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Warnf("close %s failed: %v", path, err)
		}
	}()

	scanner := bufio.NewScanner(f)

	counter := 0
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if len(line) > 640 {
			// Line too long, 640 chars should be enough for anyone
			continue
		}
		processLine(line)
		counter++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}

	log.Debugf("%d lines of search history processed from ~/%s", counter, filename)
	return nil
}

// If there are duplicates, retain only the last of each
func cleanSearchHistory(history []string) []string {
	if history == nil {
		return nil
	}

	seen := make(map[string]bool)
	cleaned := make([]string, 0, len(history))
	cleanCount := 0

	// Iterate backwards to keep the last occurrence
	for i := len(history) - 1; i >= 0; i-- {
		entry := history[i]
		if !seen[entry] {
			seen[entry] = true
			cleaned = append(cleaned, entry)
		} else {
			cleanCount++
		}
	}

	// Reverse the cleaned slice to restore original order
	for i, j := 0, len(cleaned)-1; i < j; i, j = i+1, j-1 {
		cleaned[i], cleaned[j] = cleaned[j], cleaned[i]
	}

	log.Debugf("Removed %d redundant search history lines", cleanCount)
	return cleaned
}
