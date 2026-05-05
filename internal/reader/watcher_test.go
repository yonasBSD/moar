package reader

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/walles/moor/v2/internal/linemetadata"
	"gotest.tools/v3/assert"
)

func TestReadUpdatingFile(t *testing.T) {
	// Make a temp file containing one line of text, ending with a newline
	file, err := os.CreateTemp("", "moor-TestReadUpdatingFile-*.txt")
	assert.NilError(t, err)
	defer os.Remove(file.Name()) //nolint:errcheck

	const firstLineString = "First line\n"
	_, err = file.WriteString(firstLineString)
	assert.NilError(t, err)

	// Start a reader on that file
	testMe, err := NewFromFilename(file.Name(), formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	// Wait for the reader to finish reading
	assert.NilError(t, testMe.Wait())
	assert.Equal(t, len([]byte(firstLineString)), int(testMe.bytesCount))

	// Verify we got the single line
	allLines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 1)
	assert.Equal(t, testMe.GetLineCount(), 1)
	assert.Equal(t, allLines.Lines[0].Plain(), "First line")

	// Append a line to the file
	const secondLineString = "Second line\n"
	_, err = file.WriteString(secondLineString)
	assert.NilError(t, err)

	// Give the reader some time to react
	for range 20 {
		allLines := testMe.GetLines(linemetadata.Index{}, 10)
		if len(allLines.Lines) == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify we got the two lines
	allLines = testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 2, "Expected two lines after adding a second one, got %d", len(allLines.Lines))
	assert.Equal(t, testMe.GetLineCount(), 2)
	assert.Equal(t, allLines.Lines[0].Plain(), "First line")
	assert.Equal(t, allLines.Lines[1].Plain(), "Second line")

	assert.Equal(t, int(testMe.bytesCount), len([]byte(firstLineString+secondLineString)))

	// Append a third line to the file. We want to verify line 2 didn't just
	// succeed due to special handling.
	const thirdLineString = "Third line\n"
	_, err = file.WriteString(thirdLineString)
	assert.NilError(t, err)

	// Give the reader some time to react
	for i := 0; i < 20; i++ {
		allLines = testMe.GetLines(linemetadata.Index{}, 10)
		if len(allLines.Lines) == 3 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify we got all three lines
	allLines = testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 3, "Expected three lines after adding a third one, got %d", len(allLines.Lines))
	assert.Equal(t, testMe.GetLineCount(), 3)
	assert.Equal(t, allLines.Lines[0].Plain(), "First line")
	assert.Equal(t, allLines.Lines[1].Plain(), "Second line")
	assert.Equal(t, allLines.Lines[2].Plain(), "Third line")

	assert.Equal(t, int(testMe.bytesCount), len([]byte(firstLineString+secondLineString+thirdLineString)))
}

// If a file is rewritten (shrunk and replaced with new content), tailing should
// detect the shrink and reload the file rather than giving up.
func TestReadShrunkFile(t *testing.T) {
	// Make a temp file with an initial line
	file, err := os.CreateTemp("", "moor-TestReadShrunkFile-*.txt")
	assert.NilError(t, err)
	defer os.Remove(file.Name()) //nolint:errcheck

	const firstLineString = "First line\n"
	_, err = file.WriteString(firstLineString)
	assert.NilError(t, err)

	// Start a reader on that file
	testMe, err := NewFromFilename(file.Name(), formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	// Wait for the reader to finish reading
	assert.NilError(t, testMe.Wait())

	allLines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 1)
	assert.Equal(t, allLines.Lines[0].Plain(), "First line")

	// Rewrite the file with shorter content, so the file shrinks
	err = os.WriteFile(file.Name(), []byte("New\n"), 0600)
	assert.NilError(t, err)

	// Give the background tailing goroutine up to 2s to detect the shrink and reload
	for range 20 {
		allLines = testMe.GetLines(linemetadata.Index{}, 10)
		if len(allLines.Lines) == 1 && allLines.Lines[0].Plain() == "New" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for the reload to fully complete before asserting
	assert.NilError(t, testMe.Wait())

	allLines = testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 1, "Expected one line after rewrite, got %d", len(allLines.Lines))
	assert.Equal(t, allLines.Lines[0].Plain(), "New")
}

// If people keep appending to the currently opened file we should display those
// changes.
//
// This test verifies it with an initially empty file.
func TestReadUpdatingFile_InitiallyEmpty(t *testing.T) {
	// Make a temp file containing one line of text, ending with a newline
	file, err := os.CreateTemp("", "moor-TestReadUpdatingFile_NoNewlineAtEOF-*.txt")
	assert.NilError(t, err)
	defer os.Remove(file.Name()) //nolint:errcheck

	// Start a reader on that file
	testMe, err := NewFromFilename(file.Name(), formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	// Wait for the reader to finish reading
	assert.NilError(t, testMe.Wait())

	// Verify no lines
	allLines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 0)
	assert.Equal(t, testMe.GetLineCount(), 0)

	// Append a line to the file
	_, err = file.WriteString("Text\n")
	assert.NilError(t, err)

	// Give the reader some time to react
	for i := 0; i < 20; i++ {
		allLines := testMe.GetLines(linemetadata.Index{}, 10)
		if len(allLines.Lines) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify we got the two lines
	allLines = testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 1, "Expected one line after adding one, got %d", len(allLines.Lines))
	assert.Equal(t, testMe.GetLineCount(), 1)
	assert.Equal(t, allLines.Lines[0].Plain(), "Text")
}

// If people keep appending to the currently opened file we should display those
// changes.
//
// This test verifies it with the initial contents not ending with a linefeed.
func TestReadUpdatingFile_HalfLine(t *testing.T) {
	// Make a temp file containing one line of text, ending with a newline
	file, err := os.CreateTemp("", "moor-TestReadUpdatingFile-*.txt")
	assert.NilError(t, err)
	defer os.Remove(file.Name()) //nolint:errcheck

	_, err = file.WriteString("Start")
	assert.NilError(t, err)

	// Start a reader on that file
	testMe, err := NewFromFilename(file.Name(), formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	// Wait for the reader to finish reading
	assert.NilError(t, testMe.Wait())
	assert.Equal(t, int(testMe.bytesCount), len([]byte("Start")))

	// Append the rest of the line
	const secondLineString = ", end\n"
	_, err = file.WriteString(secondLineString)
	assert.NilError(t, err)

	// Give the reader some time to react
	for i := 0; i < 20; i++ {
		allLines := testMe.GetLines(linemetadata.Index{}, 10)
		if len(allLines.Lines) == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify we got the two lines
	allLines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 1, "Still expecting one line, got %d", len(allLines.Lines))
	assert.Equal(t, testMe.GetLineCount(), 1)
	assert.Equal(t, allLines.Lines[0].Plain(), "Start, end")

	assert.Equal(t, int(testMe.bytesCount), len([]byte("Start, end\n")))
}

// If people keep appending to the currently opened file we should display those
// changes.
//
// This test verifies it with the initial contents ending in the middle of an UTF-8 character.
func TestReadUpdatingFile_HalfUtf8(t *testing.T) {
	// Make a temp file containing one line of text, ending with a newline
	file, err := os.CreateTemp("", "moor-TestReadUpdatingFile-*.txt")
	assert.NilError(t, err)
	defer os.Remove(file.Name()) //nolint:errcheck

	// Write "h" and half an "ä" to the file
	_, err = file.Write([]byte("här"[0:2]))
	assert.NilError(t, err)

	// Start a reader on that file
	testMe, err := NewFromFilename(file.Name(), formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	// Wait for the reader to finish reading
	assert.NilError(t, testMe.Wait())
	assert.Equal(t, testMe.GetLineCount(), 1)

	// Append the rest of the UTF-8 character
	_, err = file.WriteString("här"[2:])
	assert.NilError(t, err)

	// Give the reader some time to react
	for range 20 {
		allLines := testMe.GetLines(linemetadata.Index{}, 10)
		if len(allLines.Lines) == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify we got the two lines
	allLines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(allLines.Lines), 1, "Still expecting one line, got %d", len(allLines.Lines))
	assert.Equal(t, testMe.GetLineCount(), 1)
	assert.Equal(t, allLines.Lines[0].Plain(), "här")

	assert.Equal(t, int(testMe.bytesCount), len([]byte("här")))
}

func TestTailCompressedFileNoReloadLoop(t *testing.T) {
	// Create some highly compressible text
	var strBuilder strings.Builder
	for i := 0; i < 100; i++ {
		strBuilder.WriteString("this is a repetitive string that compresses very well\n")
	}
	uncompressedText := []byte(strBuilder.String())

	// Compress it
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(uncompressedText)
	assert.NilError(t, err)
	err = gz.Close()
	assert.NilError(t, err)

	compressedData := buf.Bytes()

	// Create a temporary directory and file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "testlog.gz")
	err = os.WriteFile(tempFile, compressedData, 0644)
	assert.NilError(t, err)

	// Make sure decompressed bytes are greater than physical file size,
	// otherwise the bug won't trigger.
	if len(compressedData) >= len(uncompressedText) {
		t.Fatalf("Test setup failure: compressed size (%d) >= uncompressed bytes (%d)", len(compressedData), len(uncompressedText))
	}

	// Initialize the Reader
	options := ReaderOptions{}
	reader, err := NewFromFilename(tempFile, nil, options)
	assert.NilError(t, err)
	defer reader.Close()

	// Wait for it to read the file
	err = reader.Wait()
	assert.NilError(t, err)

	// Capture the first line to see if it changes
	firstLineIndex := linemetadata.IndexFromZeroBased(0)
	lineBefore := reader.GetLine(firstLineIndex)

	// Simulate an iteration of the tail polling loop (which triggers the bug)
	shouldContinue, err := reader.tailOnce()
	assert.NilError(t, err)
	if shouldContinue {
		t.Fatalf("tailOnce was expected to stop tailing compressed files, but it continued")
	}

	// Sleep slightly on the offchance it's reading async
	time.Sleep(100 * time.Millisecond)
	err = reader.Wait()
	assert.NilError(t, err)

	lineAfter := reader.GetLine(firstLineIndex)

	// We compare the literal Line struct pointers
	if lineBefore.Line != lineAfter.Line {
		t.Errorf("FAIL: Reader reloaded the file! Expected pointers to match.")
	}
}
