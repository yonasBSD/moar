package main

import (
	log "github.com/sirupsen/logrus"
)

// Implements twin.Logger by forwarding to logrus
type twinLogger struct{}

func (l *twinLogger) Debug(msg string) {
	log.Debug(msg)
}

func (l *twinLogger) Info(msg string) {
	log.Info(msg)
}

func (l *twinLogger) Error(msg string) {
	log.Error(msg)
}
