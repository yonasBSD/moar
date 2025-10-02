package internal

import (
	"unicode"

	"github.com/walles/moor/v2/twin"
)

type InputBoxOnTextChanged func(text string)

type AcceptMode int

const (
	INPUTBOX_ACCEPT_ALL AcceptMode = iota
	INPUTBOX_ACCEPT_POSITIVE_NUMBERS
)

type InputBox struct {
	text string

	// accept controls what input is accepted. Use the INPUTBOX_ACCEPT_*
	// constants defined above.
	accept AcceptMode

	// cursorPos is the insertion point in runes (0 == before first rune,
	// len(runes) == after last rune).
	cursorPos int

	// onTextChanged is an optional callback which is triggered when the text
	// of the InputBox changes.
	onTextChanged InputBoxOnTextChanged
}

// setText sets the text of the inputBox and triggers the onTextChanged callback.
func (b *InputBox) setText(text string) {
	b.text = text
	b.moveCursorEnd()
	if b.onTextChanged != nil {
		b.onTextChanged(b.text)
	}
}

// draw renders the input box at the bottom line of the screen, showing a
// simple prompt and the current text with a reverse attribute cursor.
func (b *InputBox) draw(screen twin.Screen, prompt string) {
	width, height := screen.Size()
	pos := 0

	// Draw the prompt first
	for _, ch := range prompt {
		pos += screen.SetCell(pos, height-1, twin.NewStyledRune(ch, twin.StyleDefault))
	}

	// Work with runes for cursor correctness
	textRunes := []rune(b.text)
	if b.cursorPos < 0 {
		b.cursorPos = 0
	}
	if b.cursorPos > len(textRunes) {
		b.cursorPos = len(textRunes)
	}

	// Draw left side (before cursor)
	for i, ch := range textRunes {
		if i == b.cursorPos {
			break
		}
		pos += screen.SetCell(pos, height-1, twin.NewStyledRune(ch, twin.StyleDefault))
	}

	// If cursor is on a rune, invert that rune. If cursor is at the end,
	// show an inverted blank cell.
	if b.cursorPos < len(textRunes) {
		pos += screen.SetCell(pos, height-1, twin.NewStyledRune(textRunes[b.cursorPos], twin.StyleDefault.WithAttr(twin.AttrReverse)))

		// Draw right side after the cursor rune
		for i := b.cursorPos + 1; i < len(textRunes); i++ {
			pos += screen.SetCell(pos, height-1, twin.NewStyledRune(textRunes[i], twin.StyleDefault))
		}
	} else {
		// Cursor at end -> reverse blank
		pos += screen.SetCell(pos, height-1, twin.NewStyledRune(' ', twin.StyleDefault.WithAttr(twin.AttrReverse)))
	}

	// Clear the rest of the line
	for pos < width {
		pos += screen.SetCell(pos, height-1, twin.NewStyledRune(' ', twin.StyleDefault))
	}
}

// insertRune appends runes to the text of the InputBox and returns if those have been processed.
// (Some keyboards send 0x08 instead of backspace, so we support it here too).
func (b *InputBox) insertRune(char rune) bool {
	if char == '\x08' {
		b.backspace()
		return true
	}
	// If configured to accept numbers only, drop any non-digit rune.
	if b.accept == INPUTBOX_ACCEPT_POSITIVE_NUMBERS {
		if !unicode.IsDigit(char) {
			return false
		}
	}

	// Insert at cursor position
	runes := []rune(b.text)
	if b.cursorPos < 0 {
		b.cursorPos = 0
	}
	if b.cursorPos > len(runes) {
		b.cursorPos = len(runes)
	}

	// Build a new rune slice with the inserted rune
	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:b.cursorPos]...)
	newRunes = append(newRunes, char)
	if b.cursorPos < len(runes) {
		newRunes = append(newRunes, runes[b.cursorPos:]...)
	}
	b.text = string(newRunes)
	b.cursorPos++

	// finally let's tell someone that the text has changed
	if b.onTextChanged != nil {
		b.onTextChanged(b.text)
	}
	return true
}

// moveCursorLeft moves the cursor one rune to the left.
func (b *InputBox) moveCursorLeft() {
	if b.cursorPos > 0 {
		b.cursorPos--
	}
}

// moveCursorRight moves the cursor one rune to the right.
func (b *InputBox) moveCursorRight() {
	if b.cursorPos < len([]rune(b.text)) {
		b.cursorPos++
	}
}

// moveCursorHome moves the cursor to the start of the text.
func (b *InputBox) moveCursorHome() {
	b.cursorPos = 0
}

// moveCursorEnd moves the cursor to the end of the text.
func (b *InputBox) moveCursorEnd() {
	b.cursorPos = len([]rune(b.text))
}

// backspace removes the rune before the cursor and moves the cursor left.
func (b *InputBox) backspace() {
	runes := []rune(b.text)
	if b.cursorPos > 0 && len(runes) > 0 {
		runes = append(runes[:b.cursorPos-1], runes[b.cursorPos:]...)
		b.cursorPos--
		b.text = string(runes)
		if b.onTextChanged != nil {
			b.onTextChanged(b.text)
		}
	}
}

// delete removes the rune at the cursor.
func (b *InputBox) delete() {
	runes := []rune(b.text)
	if b.cursorPos < len(runes) {
		runes = append(runes[:b.cursorPos], runes[b.cursorPos+1:]...)
		b.text = string(runes)
		if b.onTextChanged != nil {
			b.onTextChanged(b.text)
		}
	}
}
