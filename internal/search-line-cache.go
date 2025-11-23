package internal

import (
	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/reader"
)

// For small searches or few cores, search will be fast no matter what we put here.
// For large searches on many-core systems, a larger cache will help performance.
// To evaluate:
//
//	go test -run='^$' -bench 'Search' ./internal
//
// On Johan's laptop, 200 is better than both 100 and 400.
const searchLineCacheSize = 200

type searchLineCache struct {
	lines []*reader.NumberedLine
}

func (c *searchLineCache) getLine(reader reader.Reader, index linemetadata.Index, direction SearchDirection) *reader.NumberedLine {
	// Do we have a cache hit?
	if len(c.lines) > 0 {
		firstCachedIndexInclusive := c.lines[0].Index
		lastCachedIndexInclusive := c.lines[len(c.lines)-1].Index
		if !index.IsBefore(firstCachedIndexInclusive) && !index.IsAfter(lastCachedIndexInclusive) {
			cachedLine := c.lines[index.Index()-firstCachedIndexInclusive.Index()]
			return cachedLine
		}
	}

	// Cache miss, load new lines
	firstIndexToRequest := index
	if direction == SearchDirectionBackward {
		// Let's say we want index 10 to be in the cache. Cache size is 5.
		// Then, the first index must be 6, so that we get 6,7,8,9,10.
		// Or in other words, 10 - 5 + 1 = 6.
		firstIndexToRequest = index.NonWrappingAdd(-searchLineCacheSize + 1)
	}

	lines := reader.GetLines(firstIndexToRequest, searchLineCacheSize)
	if len(lines.Lines) == 0 {
		// No lines at all
		return nil
	}

	c.lines = lines.Lines

	// Get the line from the cache
	firstCachedIndexInclusive := c.lines[0].Index
	lastCachedIndexInclusive := c.lines[len(c.lines)-1].Index
	if !index.IsBefore(firstCachedIndexInclusive) && !index.IsAfter(lastCachedIndexInclusive) {
		cachedLine := c.lines[index.Index()-firstCachedIndexInclusive.Index()]
		return cachedLine
	}

	// The reader doesn't have that line
	return nil
}
