package main

import (
	"testing"

	"github.com/walles/moor/v2/internal/linemetadata"
	"gotest.tools/v3/assert"
)

func TestParsePlusArgs_targetLine(t *testing.T) {
	index, remaining := parsePlusArgs([]string{})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{})

	index, remaining = parsePlusArgs([]string{"+"})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{"+"})

	// Ref: https://github.com/walles/moor/issues/316
	index, remaining = parsePlusArgs([]string{"+0"})
	assert.Equal(t, *index, linemetadata.IndexFromOneBased(1))
	assert.DeepEqual(t, remaining, []string{})

	index, remaining = parsePlusArgs([]string{"+1"})
	assert.Equal(t, *index, linemetadata.IndexFromOneBased(1))
	assert.DeepEqual(t, remaining, []string{})
}
