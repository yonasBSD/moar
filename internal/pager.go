package internal

import (
	"fmt"
	"math"
	"regexp"
	"runtime/debug"
	"sync"
	"time"

	"github.com/alecthomas/chroma/v2"
	log "github.com/sirupsen/logrus"
	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/reader"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
)

type PagerMode interface {
	onKey(key twin.KeyCode)
	onRune(char rune)
	drawFooter(statusText string, spinner string)
}

type StatusBarOption int

const (
	//revive:disable-next-line:var-naming
	STATUSBAR_STYLE_INVERSE StatusBarOption = iota
	//revive:disable-next-line:var-naming
	STATUSBAR_STYLE_PLAIN
	//revive:disable-next-line:var-naming
	STATUSBAR_STYLE_BOLD
)

type eventSpinnerUpdate struct {
	spinner string
}

type eventMoreLinesAvailable struct{}

// Either reading, highlighting or both are done. Check reader.Done() and
// reader.HighlightingDone() for details.
type eventMaybeDone struct{}

// Pager is the main on-screen pager
type Pager struct {
	readers       []*reader.ReaderImpl // Immutable since startup, no locking needed
	currentReader int                  // Index into the readers slice
	readerLock    sync.Mutex           // Protects currentReader

	readerSwitched chan struct{}

	// A view of the current reader, possibly filtered
	filteringReader FilteringReader

	screen              twin.Screen
	quit                bool
	scrollPosition      scrollPosition
	leftColumnZeroBased int

	// Maybe this should be renamed to "controller"? Because it controls the UI?
	// But since we replace it in a lot of places based on the UI mode, maybe
	// mode is better?
	mode PagerMode

	searchString  string
	searchPattern *regexp.Regexp
	filterPattern *regexp.Regexp

	// We used to have a "Following" field here. If you want to follow, set
	// TargetLineNumber to LineNumberMax() instead, see below.

	isShowingHelp bool
	preHelpState  *_PreHelpState

	// User preference
	ShowLineNumbers bool

	// Current state, initialized in StartPaging()
	showLineNumbers bool

	StatusBarStyle StatusBarOption
	ShowStatusBar  bool

	UnprintableStyle textstyles.UnprintableStyleT

	WrapLongLines bool

	// Ref: https://github.com/walles/moor/issues/113
	QuitIfOneScreen bool

	// Ref: https://github.com/walles/moor/issues/94
	ScrollLeftHint  textstyles.CellWithMetadata
	ScrollRightHint textstyles.CellWithMetadata

	SideScrollAmount int // Left / right arrow keys scroll amount

	TabSize int // Number of spaces per tab, default 8, should be positive

	// If non-nil, scroll to this line as soon as possible. Set this value to
	// IndexMax() to follow the end of the input (tail).
	//
	// NOTE: Always use setTargetLine() to keep the reader in sync with the
	// pager!
	TargetLine *linemetadata.Index

	// If true, pager will clear the screen on return. If false, pager will
	// clear the last line, and show the cursor.
	DeInit bool

	// If DeInit is false, leave this number of lines for the shell prompt after
	// exiting
	DeInitFalseMargin int

	WithTerminalFg bool // If true, don't set linePrefix

	// Length of the longest line displayed. This is used for limiting scrolling to the right.
	longestLineLength int

	// Bookmarks that you can come back to.
	//
	// Ref: https://github.com/walles/moor/issues/175
	bookmarks map[rune]scrollPosition

	AfterExit func() error
}

type _PreHelpState struct {
	scrollPosition      scrollPosition
	leftColumnZeroBased int
	targetLine          *linemetadata.Index
}

var _HelpReader = reader.NewFromTextForTesting("Help", `
Welcome to Moor, the nice pager!

Miscellaneous
-------------
* Press 'q' or 'ESC' to quit
* Press 'w' to toggle wrapping of long lines
* Press '=' to toggle showing the status bar at the bottom
* Press 'v' to edit the file in your favorite editor
* Press CTRL-t to change the tab size

Moving around
-------------
* Arrow keys
* Alt key plus left / right arrow steps one column at a time
* Left / right can be used to hide / show line numbers
* Home and End for start / end of the document
* 'g' for going to a specific line number
* 'm' sets a mark, you will be asked for a letter to label it with
* ' (single quote) jumps to the mark
* CTRL-p moves to the previous line
* CTRL-n moves to the next line
* PageUp / 'b' and PageDown / 'f'
* SPACE moves down a page
* < / 'gg' to go to the start of the document
* > / 'G' to go to the end of the document
* Half page 'u'p / 'd'own, or CTRL-u / CTRL-d
* CTRL-a moves to the leftmost position
* RETURN moves down one line

Switching files (if you opened multiple files)
----------------------------------------------
* Press ':' to enter file switching mode

Filtering
---------
Type '&' to start filtering, then type your filter expression.

While filtering, arrow keys, PageUp, PageDown, Home and End work as usual.

Press 'ESC' or RETURN to exit filtering mode.

Searching
---------
* Type / to start searching, then type what you want to find
* Type ? to search backwards, then type what you want to find
* Type RETURN to stop searching, or ESC to skip back to where the search started
* Find next by typing 'n' (for "next")
* Find previous by typing SHIFT-N or 'p' (for "previous")
* Search is case sensitive if it contains any UPPER CASE CHARACTERS
* Search is interpreted as a regexp if it is a valid one

Reporting bugs
--------------
File issues at https://github.com/walles/moor/issues, or post
questions to johan.walles@gmail.com.

Installing Moor as your default pager
-------------------------------------
Put the following line in your ~/.bashrc, ~/.bash_profile or ~/.zshrc:
  export PAGER=moor

Source Code
-----------
Available at https://github.com/walles/moor/.
`)

// NewPager creates a new Pager with default settings
func NewPager(readers ...*reader.ReaderImpl) *Pager {
	if len(readers) == 0 {
		panic("NewPager() needs at least one reader")
	}

	var name string
	if readers[0] == nil || readers[0].DisplayName == nil || len(*readers[0].DisplayName) == 0 {
		name = "Pager"
	} else {
		name = "Pager " + *readers[0].DisplayName
	}

	pager := Pager{
		readers:          readers,
		currentReader:    0,
		readerSwitched:   make(chan struct{}, 1),
		quit:             false,
		ShowLineNumbers:  true, // Constant throghout the lifetime of the pager
		showLineNumbers:  true, // Will be updated over time
		ShowStatusBar:    true,
		DeInit:           true,
		SideScrollAmount: 16,
		TabSize:          8, // This is what less defaults to
		ScrollLeftHint:   textstyles.CellWithMetadata{Rune: '<', Style: twin.StyleDefault.WithAttr(twin.AttrReverse)},
		ScrollRightHint:  textstyles.CellWithMetadata{Rune: '>', Style: twin.StyleDefault.WithAttr(twin.AttrReverse)},
		scrollPosition:   newScrollPosition(name),
	}

	pager.mode = PagerModeViewing{pager: &pager}
	pager.filteringReader = FilteringReader{
		BackingReader: readers[0], // Always start with the first reader
		FilterPattern: &pager.filterPattern,
	}

	return &pager
}

// How many lines are visible on screen? Depends on screen height and whether or
// not the status bar is visible.
func (p *Pager) visibleHeight() int {
	_, height := p.screen.Size()

	// Only the viewing mode can be without status bar
	hasStatusBar := p.ShowStatusBar || !p.isViewing()

	if hasStatusBar {
		return height - 1
	}

	return height
}

// How many cells are needed for this line number? Includes padding.
//
// Returns 0 if line numbers are disabled.
func (p *Pager) getLineNumberPrefixLength(lineNumber linemetadata.Number) int {
	if !p.showLineNumbers {
		return 0
	}

	length := len(lineNumber.Format()) + 1 // +1 for the space after the line number

	if length < 4 {
		// 4 = space for 3 digits followed by one whitespace
		//
		// https://github.com/walles/moor/issues/38
		return 4
	}

	return length
}

// Draw the footer string at the bottom using the status bar style.
//
// Single quoted parts of the help text will be bolded.
//
// footer example value: "file.txt: 123 lines  0%"
// help example value: "Press 'h' for help, 'q' to quit"
func (p *Pager) setFooter(footer string, help string) {
	width, height := p.screen.Size()

	pos := 0

	// File name and percentage, no keyboard shortcut highlighting
	for _, token := range footer + "  " {
		pos += p.screen.SetCell(pos, height-1, twin.NewStyledRune(token, statusbarStyle))
	}

	// Help text, highlight keyboard shortcuts
	highlightAttr := twin.AttrBold
	if statusbarStyle.HasAttr(highlightAttr) {
		highlightAttr = twin.AttrUnderline
	}
	if statusbarStyle.HasAttr(highlightAttr) {
		highlightAttr = twin.AttrReverse
	}
	style := statusbarStyle
	for _, token := range help {
		if token == '\'' {
			// Highlight things within single quotes
			if style == statusbarStyle {
				style = statusbarStyle.WithAttr(highlightAttr)
			} else {
				style = statusbarStyle
			}
			continue
		}
		pos += p.screen.SetCell(pos, height-1, twin.NewStyledRune(token, style))
	}

	for pos < width {
		pos += p.screen.SetCell(pos, height-1, twin.NewStyledRune(' ', statusbarStyle))
	}
}

// Quit leaves the help screen or quits the pager
func (p *Pager) Quit() {
	if !p.isShowingHelp {
		p.quit = true
		return
	}

	// Reset help
	p.isShowingHelp = false
	p.scrollPosition = p.preHelpState.scrollPosition
	p.leftColumnZeroBased = p.preHelpState.leftColumnZeroBased
	p.setTargetLine(p.preHelpState.targetLine)
	p.preHelpState = nil
}

// Negative deltas move left instead
func (p *Pager) moveRight(delta int) {
	if p.showLineNumbers && delta > 0 {
		p.showLineNumbers = false
		return
	}

	if p.leftColumnZeroBased == 0 && delta < 0 {
		p.showLineNumbers = true
		return
	}

	result := p.leftColumnZeroBased + delta
	if result < 0 {
		p.leftColumnZeroBased = 0
	} else {
		p.leftColumnZeroBased = result
	}

	// If we try to move past the characters when moving right, stop scrolling to
	// avoid moving infinitely into the void.
	if p.leftColumnZeroBased > p.longestLineLength {
		p.leftColumnZeroBased = p.longestLineLength
	}
}

func (p *Pager) Reader() reader.Reader {
	if p.isShowingHelp {
		return _HelpReader
	}
	return &p.filteringReader
}

func (p *Pager) handleScrolledUp() {
	p.setTargetLine(nil)
}

func (p *Pager) handleScrolledDown() {
	if p.isScrolledToEnd() {
		// Follow output
		reallyHigh := linemetadata.IndexMax()
		p.setTargetLine(&reallyHigh)
	} else {
		p.setTargetLine(nil)
	}
}

// Except for setting TargetLine, this method also syncs with the reader so that
// the reader knows how many lines it needs to fetch.
func (p *Pager) setTargetLine(targetLine *linemetadata.Index) {
	p.readerLock.Lock()
	r := p.readers[p.currentReader]
	p.readerLock.Unlock()

	log.Trace("Pager: Setting target line to ", targetLine, "...")
	p.TargetLine = targetLine
	if targetLine == nil {
		// No target, just do your thing
		r.SetPauseAfterLines(reader.DEFAULT_PAUSE_AFTER_LINES)
		return
	}

	// The value 1000 here is supposed to be larger than any possible screen
	// height, to give us some lookahead and to avoid fetching too few lines.
	targetValue := targetLine.Index() + 1000
	if targetValue < targetLine.Index() {
		// Overflow detected, clip to max int
		targetValue = math.MaxInt
	}
	if targetValue < reader.DEFAULT_PAUSE_AFTER_LINES {
		targetValue = reader.DEFAULT_PAUSE_AFTER_LINES
	}
	r.SetPauseAfterLines(targetValue)
}

// StartPaging brings up the pager on screen
func (p *Pager) StartPaging(screen twin.Screen, chromaStyle *chroma.Style, chromaFormatter *chroma.Formatter) {
	log.Info("Pager starting")

	defer func() {
		p.readerLock.Lock()
		r := p.readers[p.currentReader]
		p.readerLock.Unlock()

		if r.Err != nil {
			log.Warnf("Reader reported an error: %s", r.Err.Error())
		}
	}()

	p.showLineNumbers = p.ShowLineNumbers

	textstyles.UnprintableStyle = p.UnprintableStyle
	if p.TabSize > 0 {
		// "0" = unset, stay at the default. If the tab size is negative, just
		// ignoring it seems like the right move.
		textstyles.TabSize = p.TabSize
	}
	consumeLessTermcapEnvs(screen.TerminalBackground(), chromaStyle, chromaFormatter)
	styleUI(screen.TerminalBackground(), chromaStyle, chromaFormatter, p.StatusBarStyle, p.WithTerminalFg)

	p.screen = screen
	p.mode = PagerModeViewing{pager: p}
	p.bookmarks = make(map[rune]scrollPosition)

	// Make sure the reader knows how many lines we want
	p.setTargetLine(p.TargetLine)

	go func() {
		defer func() {
			PanicHandler("StartPaging()/goroutine", recover(), debug.Stack())
		}()

		spinnerFrames := [...]string{"/.\\", "-o-", "\\O/", "| |"}
		spinnerIndex := 0
		spinnerTicker := time.NewTicker(200 * time.Millisecond)
		lastSpinnerFrame := "UNSET" // Track the last spinner frame to avoid unnecessary redraws

		// Support throttling of more-lines-available reads, see below
		p.readerLock.Lock()
		throttledMoreLines := p.readers[p.currentReader].MoreLinesAdded
		p.readerLock.Unlock()

		var reenable <-chan time.Time

		for {
			p.readerLock.Lock()
			r := p.readers[p.currentReader]
			p.readerLock.Unlock()

			select {
			case <-p.readerSwitched:
				// A different reader is now active
				p.filterPattern = nil

				p.readerLock.Lock()
				r = p.readers[p.currentReader]
				p.filteringReader.SetBackingReader(r)
				p.readerLock.Unlock()

				// Look in the right place for more lines
				throttledMoreLines = r.MoreLinesAdded
				reenable = nil

				// Reset spinner for new reader so that we show it again if needed
				lastSpinnerFrame = "UNSET"

				// Tell the viewer to replace the view
				screen.Events() <- eventMoreLinesAvailable{}

			case <-throttledMoreLines:
				screen.Events() <- eventMoreLinesAvailable{}

				// Disable further receives for 200ms. This avoids flooding the
				// event loop if a lot of lines are added in a short time.
				throttledMoreLines = nil
				reenable = time.After(200 * time.Millisecond)

			case <-reenable:
				// Re-enable channel
				throttledMoreLines = r.MoreLinesAdded
				reenable = nil

			case <-spinnerTicker.C:
				currentSpinnerFrame := spinnerFrames[spinnerIndex]
				if r.Done.Load() {
					// We're done, clear the spinner
					currentSpinnerFrame = ""
				}

				spinnerIndex++
				if spinnerIndex >= len(spinnerFrames) {
					spinnerIndex = 0
				}

				if currentSpinnerFrame == lastSpinnerFrame {
					// Prevent unnecessary redraws
					continue
				}

				screen.Events() <- eventSpinnerUpdate{currentSpinnerFrame}
				lastSpinnerFrame = currentSpinnerFrame

			case <-r.MaybeDone:
				screen.Events() <- eventMaybeDone{}
			}
		}
	}()

	log.Info("Entering pager main loop...")

	// Main loop
	spinner := ""
	for !p.quit {
		if len(screen.Events()) == 0 {
			// Nothing more to process for now, redraw the screen
			p.redraw(spinner)

			p.readerLock.Lock()
			r := p.readers[p.currentReader]
			p.readerLock.Unlock()

			// Ref:
			// https://github.com/gwsw/less/blob/ff8869aa0485f7188d942723c9fb50afb1892e62/command.c#L828-L831
			//
			// Note that we do the slow (atomic) checks only if the fast ones
			// (no locking required) passed.
			//
			// Also, we only do this if we have exactly one reader, because
			// that's what less does.
			if len(p.readers) == 1 && p.QuitIfOneScreen && !p.isShowingHelp && r.Done.Load() && r.HighlightingDone.Load() {
				if p.fitsOnOneScreen() {
					// Ref:
					// https://github.com/walles/moor/issues/113#issuecomment-1368294132
					p.showLineNumbers = false // Requires a redraw to take effect, see below
					p.DeInit = false
					p.quit = true

					// Without this the line numbers setting ^ won't take effect
					p.redraw(spinner)

					log.Info("Exiting because of --quit-if-one-screen, everything fit on one screen and we're done")

					break
				}
			}
		}

		event := <-screen.Events()
		switch event := event.(type) {
		case twin.EventKeyCode:
			log.Tracef("Handling key event %d...", event.KeyCode())
			p.mode.onKey(event.KeyCode())

		case twin.EventRune:
			log.Tracef("Handling rune event '%c'/0x%04x...", event.Rune(), event.Rune())
			p.mode.onRune(event.Rune())

		case twin.EventMouse:
			log.Tracef("Handling mouse event %d...", event.Buttons())
			switch event.Buttons() {
			case twin.MouseWheelUp:
				// Clipping is done in _Redraw()
				p.scrollPosition = p.scrollPosition.PreviousLine(1)

			case twin.MouseWheelDown:
				// Clipping is done in _Redraw()
				p.scrollPosition = p.scrollPosition.NextLine(1)

			case twin.MouseWheelLeft:
				p.moveRight(-p.SideScrollAmount)

			case twin.MouseWheelRight:
				p.moveRight(p.SideScrollAmount)
			}

		case twin.EventResize:
			// We'll be implicitly redrawn just by taking another lap in the loop

		case twin.EventExit:
			log.Info("Got a Twin exit event, exiting")
			return

		case eventMoreLinesAvailable:
			if p.TargetLine != nil {
				// The user wants to scroll down to a specific line number
				if linemetadata.IndexFromLength(p.Reader().GetLineCount()).IsBefore(*p.TargetLine) {
					// Not there yet, keep scrolling
					p.scrollToEnd()
				} else {
					// We see the target, scroll to it
					p.scrollPosition = NewScrollPositionFromIndex(*p.TargetLine, "goToTargetLine")
					p.setTargetLine(nil)
				}
			}

		case eventMaybeDone:
			// Do nothing. We got this just so that we'll do the QuitIfOneScreen
			// check (above) as soon as highlighting is done.

		case eventSpinnerUpdate:
			spinner = event.spinner

		default:
			log.Warnf("Unhandled event type: %v", event)
		}
	}
}

// The height parameter is the terminal height minus the height of the user's
// shell prompt.
//
// This way nothing gets scrolled off screen after we exit.
func (p *Pager) fitsOnOneScreenWrapped() bool {
	if len(p.readers) != 1 {
		// At most one screen will fit on one screen...
		return false
	}

	// Create a fake screen of height + 1 lines
	width, height := p.screen.Size()

	// If the screen height is one, and the prompt height is zero, then the last
	// line number will be zero. But since we want one extra line to check for
	// overflow, we now want the last line number to be one.
	//
	// So if the initial height is 1, we want the last line number to be 1.
	// Which is what we get from here.
	lastScreenRow := height - p.DeInitFalseMargin

	// If the last screen row is supposed to be one, we need to set the height
	// to two. So we add one here.
	testScreenHeight := lastScreenRow + 1
	testScreen := twin.NewFakeScreen(width, testScreenHeight)

	// Create a fake pager for that screen, with no status bar, and matching
	// line number settings + tab size and wrap settings
	p.readerLock.Lock()
	fakePager := NewPager(p.readers[0])
	p.readerLock.Unlock()
	fakePager.screen = testScreen

	// If we drop out because of quit-if-one-screen, we will not print any line numbers
	fakePager.showLineNumbers = false

	fakePager.WrapLongLines = p.WrapLongLines
	fakePager.ShowStatusBar = false // We are only interested in content lines
	fakePager.TabSize = p.TabSize

	// Render on our test screen
	rendered := fakePager.renderLines()

	return len(rendered.lines) < testScreenHeight
}

func (p *Pager) fitsOnOneScreen() bool {
	if len(p.readers) != 1 {
		// At most one screen will fit on one screen...
		return false
	}

	if p.WrapLongLines {
		return p.fitsOnOneScreenWrapped()
	}

	width, height := p.screen.Size()
	height -= p.DeInitFalseMargin

	p.readerLock.Lock()
	reader := p.readers[0]
	p.readerLock.Unlock()

	if reader.GetLineCount() > height {
		return false
	}

	lines := reader.GetLines(linemetadata.Index{}, reader.GetLineCount())
	for _, line := range lines.Lines {
		rendered := line.HighlightedTokens(twin.StyleDefault, twin.StyleDefault, nil, nil).StyledRunes
		if len(rendered) > width {
			// This line is too long to fit on one screen line, no fit
			return false
		}
	}
	return true
}

// After the pager has exited and the normal screen has been restored, you can
// call this method to print the pager contents to screen again, faking
// "leaving" pager contents on screen after exit.
func (p *Pager) ReprintAfterExit() {
	// Figure out how many screen lines are used by pager contents
	renderedScreen := p.renderLines()
	screenLinesCount := len(renderedScreen.lines)

	_, screenHeight := p.screen.Size()
	screenHeightWithoutFooter := screenHeight - p.DeInitFalseMargin
	if screenLinesCount > screenHeightWithoutFooter {
		screenLinesCount = screenHeightWithoutFooter
	}

	if screenLinesCount > 0 {
		p.screen.ShowNLines(screenLinesCount)
		fmt.Println()
	}
}
