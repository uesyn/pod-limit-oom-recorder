package worker

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"k8s.io/klog/v2"
)

var wg *sync.WaitGroup = &sync.WaitGroup{}
var workers []func(chan struct{}) = make([]func(chan struct{}), 0)
var stopCh = make(chan struct{}, 0)

func handleSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	klog.Info("signal received...")
	close(stopCh)
}

func Add(f func(stopCh chan struct{})) {
	wg.Add(1)
	workers = append(workers, f)
}

func Start() {
	go handleSignal()

	for _, w := range workers {
		f := w
		go func() {
			defer wg.Done()
			f(stopCh)
		}()
	}
}

func Wait() {
	wg.Wait()
}
