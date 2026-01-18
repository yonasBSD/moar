package textstyles

import "unicode/utf8"

// Length of the long consumeBullet() pattern
const lookaheadBufferSize = 7

// Lazily iterate a string by runes. Init by setting str only.
type lazyRunes struct {
	str string

	// From what position in the string should we pick the next rune?
	nextByteIndex int

	lookaheadBuffer []rune
}

func (l *lazyRunes) getRelative(runeIndex int) *rune {
	FIXME
}

func (l *lazyRunes) hasNext() bool {
	return l.nextByteIndex < len(l.str)
}

func (l *lazyRunes) next() {
	if l.lookaheadBuffer == nil {
		// We're just getting started
		l.lookaheadBuffer = make([]rune, 0, lookaheadBufferSize)
	}

	nextRune, runeSize := utf8.DecodeRuneInString(l.str[l.nextByteIndex:])
	if runeSize == 0 {
		// We are done
		return
	}

	// Shift the lookahead buffer down
	if len(l.lookaheadBuffer) == cap(l.lookaheadBuffer) {
		l.lookaheadBuffer = l.lookaheadBuffer[1:]
	}
	l.lookaheadBuffer = append(l.lookaheadBuffer, nextRune)

	l.nextByteIndex += runeSize
}
