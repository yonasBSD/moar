package internal

import (
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	log "github.com/sirupsen/logrus"
	"github.com/walles/moor/v2/twin"
)

const defaultDarkTheme = "native"

// I decided on a light theme by doing this:
//
//	rg -i 'Background.*bg:#ffffff' | rg -v '[^:]#[^0]' | sort | cut -d: -f1 | xargs rg --files-without-match '"LiteralString".*bg:' |  xargs wc -l | sort -r | grep -v tango
//
// Then I picked github because it has a bright background, a dark foreground
// and it looks OK on a white terminal with an unmodified color palette.
//
// This used to be tango, which I mostly like, but it comes with underlined
// whitespace which looks weird when we change the background color of lines
// with search hits:
//
// https://github.com/alecthomas/chroma/blob/daa879b239442af21c3e62517a9da8f11d1c15b2/styles/tango.xml#L71
const defaultLightTheme = "github"

// Checks the terminal background color and returns either a dark or light theme
func GetStyleForScreen(screen twin.Screen) chroma.Style {
	var style = *styles.Get(defaultDarkTheme)

	t0 := time.Now()
	screen.RequestTerminalBackgroundColor()
	select {
	case event := <-screen.Events():
		// Event received, let's see if it's the one we want
		switch ev := event.(type) {

		case twin.EventTerminalBackgroundDetected:
			log.Debug("Terminal background color detected as ", ev.Color, " after ", time.Since(t0))

			distanceToBlack := ev.Color.Distance(twin.NewColor24Bit(0, 0, 0))
			distanceToWhite := ev.Color.Distance(twin.NewColor24Bit(255, 255, 255))
			if distanceToBlack < distanceToWhite {
				style = *styles.Get(defaultDarkTheme)
			} else {
				style = *styles.Get(defaultLightTheme)
			}

		default:
			log.Debugf("Expected terminal background color event but got %#v after %s, putting back and giving up", ev, time.Since(t0))
			screen.Events() <- event
		}

	// The worst number I have measured was around 15ms, in GNOME Terminal
	// running inside of VirtualBox. 3x that should be enough for everyone
	// (TM).
	case <-time.After(50 * time.Millisecond):
		log.Debug("Terminal background color still not detected after ", time.Since(t0), ", giving up")
	}

	return style
}
