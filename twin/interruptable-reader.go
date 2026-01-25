package twin

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

type interruptableReaderImpl struct {
	base *os.File

	shutdownPipeReader *os.File
	shutdownPipeWriter *os.File

	interrupted atomic.Bool
}

func (r *interruptableReaderImpl) Read(p []byte) (n int, err error) {
	for {
		if r.interrupted.Load() {
			log.Info("Interruptable reader already interrupted, returning fabricated EOF")
			return 0, io.EOF
		}

		n, err = r.read(p)

		if err == syscall.EINTR {
			// Not really a problem, we can get this on window resizes for
			// example, just try again.
			continue
		}

		return
	}
}

func (r *interruptableReaderImpl) read(p []byte) (n int, err error) {
	// "This argument should be set to the highest-numbered file descriptor in
	// any of the three sets, plus 1. The indicated file descriptors in each set
	// are checked, up to this limit"
	//
	// Ref: https://man7.org/linux/man-pages/man2/select.2.html
	nfds := r.base.Fd()
	if r.shutdownPipeReader.Fd() > nfds {
		nfds = r.shutdownPipeReader.Fd()
	}

	readFds := unix.FdSet{}
	readFds.Set(int(r.shutdownPipeReader.Fd()))
	readFds.Set(int(r.base.Fd()))

	_, err = unix.Select(int(nfds)+1, &readFds, nil, nil, nil)
	if err != nil {
		// Select failed
		return
	}

	if readFds.IsSet(int(r.shutdownPipeReader.Fd())) {
		// Shutdown requested
		closeErr := r.shutdownPipeReader.Close()
		if closeErr != nil {
			// This should never happen, but if it does we should log it
			log.Error(fmt.Sprint("Failed to close shutdown pipe reader: ", closeErr))
		} else {
			log.Info("Interruptable reader shutdown pipe reader closed, returning fabricated EOF")
		}

		err = io.EOF

		return
	}

	if readFds.IsSet(int(r.base.Fd())) {
		// Base has stuff
		return r.base.Read(p)
	}

	// Neither base nor shutdown pipe was ready, this should never happen
	return
}

func (r *interruptableReaderImpl) Interrupt() {
	r.interrupted.Store(true)

	err := r.shutdownPipeWriter.Close()
	if err != nil {
		// This should never happen, but if it does we should log it
		log.Info(fmt.Sprint("Failed to close shutdown pipe writer: ", err))
		return
	}

	log.Info("Interruptable reader interrupted")
}

func newInterruptableReader(base *os.File) (interruptableReader, error) {
	reader := interruptableReaderImpl{
		base: base,
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	reader.shutdownPipeReader = pr
	reader.shutdownPipeWriter = pw

	return &reader, nil
}
