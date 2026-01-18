package textstyles

// Lazily iterate a string by runes. Init by setting str only.
type lazyRunes struct {
	str string

	rune0                  rune
	rune0ByteIndex         int
	rune0RuneIndexOneBased int

	rune1          rune
	rune1ByteIndex int

	rune2          rune
	rune2ByteIndex int
}

func (l *lazyRunes) getRelative(runeIndex int) *rune {
	FIXME
}

func (l *lazyRunes) hasNext() bool {
	FIXME
}

func (l *lazyRunes) next() {
	FIXME
}
