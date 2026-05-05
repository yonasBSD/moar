package reader

// This file contains the logic for file watching and tailing appended bytes.

import (
	"fmt"
	"io"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
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

	reader.Lock()
	reader.lines = reader.lines[:0]
	reader.bytesCount = 0
	reader.endsWithNewline = false
	reader.Err = nil
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

// tailOnce performs one iteration of the file tailing check.
//
// Returns (shouldContinue, error): shouldContinue=false means tailing should stop.
func (reader *ReaderImpl) tailOnce() (bool, error) {
	reader.RLock()
	fileName := reader.FileName
	isCompressed := reader.IsCompressed
	reader.RUnlock()
	if fileName == nil {
		return false, nil
	}

	if isCompressed {
		// Comparing physical compressed size vs decompressed bytesCount doesn't work,
		// and we can't easily seek into compressed streams to tail them anyway.
		log.Debugf("File %s is compressed, stop tailing", *fileName)
		return false, nil
	}

	fileStats, err := os.Stat(*fileName)
	if err != nil {
		log.Debugf("Failed to stat file %s while tailing, giving up: %s", *fileName, err.Error())
		return false, nil
	}

	reader.RLock()
	bytesCount := reader.bytesCount
	reader.RUnlock()

	if bytesCount == -1 {
		log.Debugf("Bytes count unknown for %s, stop tailing", *fileName)
		return false, nil
	}

	if fileStats.Size() == bytesCount {
		log.Tracef("File %s unchanged at %d bytes, continue tailing", *fileName, fileStats.Size())
		return true, nil
	}

	if fileStats.Size() < bytesCount {
		err := reader.reloadFromFile(*fileName)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return reader.readNewBytes(*fileName, bytesCount)
}

func (reader *ReaderImpl) tailFile() error {
	reader.RLock()
	fileName := reader.FileName
	isCompressed := reader.IsCompressed
	reader.RUnlock()
	if fileName == nil {
		return nil
	}

	if isCompressed {
		log.Debugf("Giving up on tailing, %s is compressed", *fileName)
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
