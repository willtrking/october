package october

import (
	"os"
	"os/signal"
	"syscall"
)

var DefaultGracefulShutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

func OnSignal(f func(os.Signal), signals ...os.Signal) {

	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)

	go func() {
		for {
			select {
			case s := <-c:
				f(s)
			}
		}
	}()
}

func OnSignalBlock(f func(os.Signal), signals ...os.Signal) {

	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)

	for {
		select {
		case s := <-c:
			f(s)
		}
	}

}

func OnShutdown(f func(os.Signal)) {
	OnSignal(f, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func OnShutdownBlock(f func(os.Signal)) {
	OnSignalBlock(f, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}
