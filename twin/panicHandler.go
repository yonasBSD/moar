package twin

// NOTE: This file should be identical to m/panicHandler.go

func panicHandler(goroutineName string, recoverResult any, stackTrace []byte) {
	if recoverResult == nil {
		return
	}

	log.Error("Goroutine panicked: " + goroutineName + "\n" + string(stackTrace))
}
