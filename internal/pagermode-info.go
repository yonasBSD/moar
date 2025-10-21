// Show some info to the user. Fall back to viewing on any input.

package internal

import "github.com/walles/moor/v2/twin"

type PagerModeInfo struct {
	pager *Pager
	text  string
}

func (m PagerModeInfo) drawFooter(_ string, _ string) {
	m.pager.setFooter(m.text, "")
}

func (m PagerModeInfo) onKey(key twin.KeyCode) {
	m.pager.mode = PagerModeViewing{m.pager}
	m.pager.mode.onKey(key)
}

func (m PagerModeInfo) onRune(char rune) {
	m.pager.mode = PagerModeViewing{m.pager}
	m.pager.mode.onRune(char)
}
