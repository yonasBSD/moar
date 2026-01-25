//go:build !windows

package twin

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"golang.org/x/term"
)

// Subscribe to SIGWINCH signals. Compared to polling, this will reduce power
// usage in the absence of window resizes.
func (screen *UnixScreen) setupSigwinchNotification() {
	screen.sigwinch = make(chan int, 1)
	screen.sigwinch <- 0 // Trigger initial screen size query

	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		defer func() {
			panicHandler("setupSigwinchNotification()/SIGWINCH", recover(), debug.Stack())
		}()

		for {
			// Await window resize signal
			<-sigwinch

			screen.onWindowResized()
		}
	}()
}

func (screen *UnixScreen) setupTtyInTtyOut() error {
	// Dup stdout so we can close stdin in Close() without closing stdout.
	// Before this dupping, we crashed on using --quit-if-one-screen.
	//
	// Ref:https://github.com/walles/moor/issues/214
	stdoutDupFd, err := syscall.Dup(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}
	stdoutDup := os.NewFile(uintptr(stdoutDupFd), "moor-stdout-dup")

	// os.Stdout is a stream that goes to our terminal window.
	//
	// So if we read from there, we'll get input from the terminal window.
	//
	// If we just read from os.Stdin that would fail when getting data piped
	// into ourselves from some other command.
	//
	// Tested on macOS and Linux, works like a charm!
	screen.ttyIn = stdoutDup // <- YES, WE SHOULD ASSIGN STDOUT TO TTYIN

	// Set input stream to raw mode
	screen.oldTerminalState, err = term.MakeRaw(int(screen.ttyIn.Fd()))
	if err != nil {
		return err
	}

	screen.ttyOut = os.Stdout

	ttyInTerminalState, err := term.GetState(int(screen.ttyIn.Fd()))
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("ttyin terminal state: %+v", ttyInTerminalState))

	ttyOutTerminalState, err := term.GetState(int(screen.ttyOut.Fd()))
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("ttyout terminal state: %+v", ttyOutTerminalState))

	return nil
}

func (screen *UnixScreen) restoreTtyInTtyOut() error {
	return term.Restore(int(screen.ttyIn.Fd()), screen.oldTerminalState)
}
