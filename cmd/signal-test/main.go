package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

func main() {

	logrus.Info("listening")
	// Wait for signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	logrus.WithField("sig", sig).Info("caught signal")
}
