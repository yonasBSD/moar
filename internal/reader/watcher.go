package reader

// This file contains the logic for file watching and tailing appended bytes.

import (
	"fmt"
	"io"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

type tailAction int

const (
	tailActionStop tailAction = iota
	tailActionContinue
	tailActionReload
	tailActionAppend
)

// reloadFromFile clears the current content and re-reads the file from scratch.
//
// FIXME: This must only be called from the tailing goroutine. If called
// concurrently with consumeLinesFromStream(), both goroutines will interleave
// line additions and bytesCount will be wrong. Fix this before adding any
// other callers (e.g. the 'r' key).
func (reader *ReaderImpl) reloadFromFile(fileName string) error {
	log.Debugf("Reloading file %s from the beginning", fileName)

	stream, _, err := ZOpen(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file %s for reloading: %w", fileName, err)
	}

	var newStat os.FileInfo
	if fileStats, statErr := os.Stat(fileName); statErr == nil {
		newStat = fileStats
	} else {
		log.Debugf("Failed to stat file %s immediately after opening for reload: %s", fileName, statErr.Error())
	}

	reader.Lock()
	reader.lines = reader.lines[:0]
	reader.bytesCount = 0
	reader.endsWithNewline = false
	reader.Err = nil
	if newStat != nil {
		reader.lastStat = newStat
	}
	reader.ReadingDone.Store(false)
	reader.HighlightingDone.Store(false)
	reader.Unlock()

	// Signal the pager to redraw the now-empty content
	select {
	case reader.MoreLinesAdded <- true:
	default:
	}

	reader.consumeLinesFromStream(stream)
	err = stream.Close()
	if err != nil {
		return fmt.Errorf("failed to close file %s after reloading: %w", fileName, err)
	}

	reader.ReadingDone.Store(true)
	select {
	case reader.MaybeDone <- true:
	default:
	}

	reader.RLock()
	formatter := reader.formatter
	options := reader.readerOptions
	reader.RUnlock()

	if formatter != nil && options.Style != nil {
		highlightFromMemory(reader, formatter, options)
	}

	reader.HighlightingDone.Store(true)
	select {
	case reader.MaybeDone <- true:
	default:
	}

	return nil
}

// readNewBytes reads bytes appended to the file since we last read it.
//
// Returns (shouldContinue, error): shouldContinue=false means tailing should stop.
func (reader *ReaderImpl) readNewBytes(fileName string, bytesCount int64) (bool, error) {
	stream, _, err := ZOpen(fileName)
	if err != nil {
		log.Debugf("Failed to open file %s for re-reading while tailing: %s", fileName, err.Error())
		return false, nil
	}

	reader.Lock()
	if fileStats, statErr := os.Stat(fileName); statErr == nil {
		reader.lastStat = fileStats
	} else {
		log.Debugf("Failed to stat file %s immediately after opening for reading new bytes: %s", fileName, statErr.Error())
	}
	reader.Unlock()

	seekable, ok := stream.(io.ReadSeekCloser)
	if !ok {
		err = stream.Close()
		if err != nil {
			log.Debugf("Giving up on tailing, failed to close non-seekable stream from %s: %s", fileName, err.Error())
			return false, nil
		}
		log.Debugf("Giving up on tailing, file %s is not seekable", fileName)
		return false, nil
	}

	_, err = seekable.Seek(bytesCount, io.SeekStart)
	if err != nil {
		log.Debugf("Failed to seek in file %s while tailing: %s", fileName, err.Error())
		return false, nil
	}

	log.Tracef("File %s grew, reading more lines from byte %d...", fileName, bytesCount)

	reader.consumeLinesFromStream(seekable)
	err = seekable.Close()
	if err != nil {
		// This can lead to file handle leaks
		return false, fmt.Errorf("failed to close file %s after tailing: %w", fileName, err)
	}

	return true, nil
}

func determineTailAction(
	fileName string,
	isCompressed bool,
	oldStat os.FileInfo,
	newStat os.FileInfo,
	statErr error,
) tailAction {
	if statErr != nil {
		log.Debugf("Failed to stat file %s while tailing, giving up: %s", fileName, statErr.Error())
		return tailActionStop
	}

	if oldStat == nil {
		log.Debugf("Previous stat unknown for %s, stop tailing", fileName)
		return tailActionStop
	}

	oldSize := oldStat.Size()
	newSize := newStat.Size()

	if newSize < oldSize {
		log.Debugf("File %s shrunk, reloading", fileName)
		return tailActionReload
	}

	if newSize > oldSize {
		if isCompressed {
			log.Debugf("Compressed file %s grew, reloading", fileName)
			return tailActionReload
		}
		log.Debugf("File %s grew from %d to %d bytes, appending", fileName, oldSize, newSize)
		return tailActionAppend
	}

	// Invariant: File size unchanged

	if newStat.ModTime().After(oldStat.ModTime()) {
		log.Debugf("File %s was updated/rotated but size is unchanged, reloading", fileName)
		return tailActionReload
	}

	// Invariant: File size unchanged and mod time unchanged

	log.Tracef("File %s unchanged at %d bytes, continue tailing", fileName, newSize)
	return tailActionContinue
}

// tailOnce performs one iteration of the file tailing check.
//
// Returns (shouldContinue, error): shouldContinue=false means tailing should stop.
func (reader *ReaderImpl) tailOnce() (bool, error) {
	reader.RLock()
	fileName := reader.FileName
	isCompressed := reader.IsCompressed
	bytesCount := reader.bytesCount
	oldStat := reader.lastStat
	reader.RUnlock()

	if fileName == nil {
		return false, nil
	}

	newStat, statErr := os.Stat(*fileName)
	action := determineTailAction(*fileName, isCompressed, oldStat, newStat, statErr)

	switch action {
	case tailActionStop:
		return false, nil
	case tailActionContinue:
		log.Tracef("File %s unchanged at %d bytes, continue tailing", *fileName, newStat.Size())
		return true, nil
	case tailActionReload:
		err := reader.reloadFromFile(*fileName)
		if err != nil {
			return false, err
		}
		return true, nil
	case tailActionAppend:
		return reader.readNewBytes(*fileName, bytesCount)
	default:
		return false, nil
	}
}

// tailFile polls the file for changes in a loop and updates the reader.
//
// Note: This starts executing ONLY after the initial parsing is completely
// finished (see `readStream`). Because initial parsing and tailing polling run
// sequentially on the same background goroutine, there is no concurrency (and
// thus no data races) between checking for appends and parsing original lines.
func (reader *ReaderImpl) tailFile() error {
	reader.RLock()
	fileName := reader.FileName
	reader.RUnlock()
	if fileName == nil {
		return nil
	}

	if !isSeekableFile(fileName) {
		log.Debugf("Giving up on tailing, %s is not seekable", *fileName)
		return nil
	}

	log.Debugf("Tailing file %s", *fileName)

	for !reader.closed.Load() {
		// NOTE: We could use something like
		// https://github.com/fsnotify/fsnotify instead of sleeping and polling
		// here.
		time.Sleep(1 * time.Second)

		shouldContinue, err := reader.tailOnce()
		if err != nil {
			return err
		}
		if !shouldContinue {
			return nil
		}
	}

	return nil
}

func isSeekableFile(fileName *string) bool {
	if fileName == nil {
		return false
	}

	file, err := os.Open(*fileName)
	if err != nil {
		return false
	}

	defer func() {
		err := file.Close()
		if err != nil {
			log.Debugf("Failed to close %s while checking seekability: %s", *fileName, err)
		}
	}()

	_, err = file.Seek(0, io.SeekCurrent)

	return err == nil
}
