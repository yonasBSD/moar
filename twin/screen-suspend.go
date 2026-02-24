//go:build !windows

package twin

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// Suspend and wait for SIGCONT, then resume. Basically ctrl-Z handling.
//
// So this method will not return until after the process is resumed again.
func (screen *UnixScreen) suspend() error {
	cont := make(chan os.Signal, 1)
	signal.Notify(cont, syscall.SIGCONT)
	defer signal.Stop(cont)

	screen.leaveAlternateScreenSession()

	err := screen.restoreTtyInTtyOut()
	if err != nil {
		return fmt.Errorf("failed to restore terminal state before suspend: %w", err)
	}

	// kill(0) = "Send signal to all processes in the current process group"
	err = syscall.Kill(0, syscall.SIGTSTP)
	if err != nil {
		restoreRawErr := screen.restoreRawModeAfterResume()
		if restoreRawErr != nil {
			return fmt.Errorf("failed to suspend process group: %w; also failed to re-enter raw mode: %v", err, restoreRawErr)
		}

		screen.enterAlternateScreenSession()
		screen.onWindowResized()

		return fmt.Errorf("failed to suspend process group: %w", err)
	}

	// Wait for SIGCONT signal to arrive
	<-cont

	err = screen.restoreRawModeAfterResume()
	if err != nil {
		return err
	}

	screen.enterAlternateScreenSession()
	screen.onWindowResized()

	return nil
}

func (screen *UnixScreen) restoreRawModeAfterResume() error {
	terminalState, err := term.MakeRaw(int(screen.ttyIn.Fd()))
	if err != nil {
		return fmt.Errorf("failed to re-enter raw mode after suspend: %w", err)
	}

	screen.oldTerminalState = terminalState
	return nil
}
