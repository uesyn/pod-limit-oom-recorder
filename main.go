package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/spf13/pflag"
	k8s "github.com/uesyn/pod-limit-oom-recorder/kubernetes"
	"github.com/uesyn/pod-limit-oom-recorder/oom"
	"github.com/uesyn/pod-limit-oom-recorder/worker"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	"k8s.io/client-go/deprecated/scheme"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/record"
)

var (
	// overwritten in built time
	version = "0.0"
	gitRepo = "https://github.com/uesyn/pod-limit-oom-recorder"

	flags      = pflag.NewFlagSet("", pflag.ExitOnError)
	kubeconfig string
	address    string
	nodeName   string
	help       bool
)

func init() {
	flags.StringVar(&kubeconfig, "kubeconfig", "", `Path to kubeconfig file to override in-cluster configuration.`)
	flags.StringVar(&address, "address", "0.0.0.0:8080", `Address and port to listen for health check.`)
	flags.BoolVarP(&help, "help", "h", false, `Show help message.`)
	flags.StringVar(&nodeName, "node", "", `Cache only the Pods on this node.`)
	flags.Usage = usage
}

func usage() {
	fmt.Fprint(os.Stderr, `pod-limit-oom-recorder watchs the Pod oom and record it as Kubernetes Event.
Usage:
  pod-limit-oom-recorder [options]
Options:
`)
	flags.PrintDefaults()
}

func main() {
	klog.InitFlags(nil)
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	if help {
		usage()
		os.Exit(0)
	}

	klog.Infof("using build: %v - %v", gitRepo, version)

	// Listen and serve health endpoint
	worker.Add(func(stopCh chan struct{}) {
		// handle SIGINT and SIGTERM for graceful shutdown
		var server = &http.Server{Addr: address, Handler: nil}
		// for health check endpoint
		http.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(http.StatusText(http.StatusOK)))
		}))

		go func() {
			<-stopCh
			if err := server.Shutdown(context.Background()); err != nil {
				klog.Errorf("shutdown error: %v", err)
			}
		}()

		klog.Infof("listening on %s", address)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			klog.Error(err)
		}
	})

	// oom watcher
	worker.Add(func(stopCh chan struct{}) {
		restConfig, err := k8s.GetKubeRestConfig(kubeconfig)
		if err != nil {
			klog.Exitf("couldn't get clientConfig: %v", err)
		}

		client := kubernetes.NewForConfigOrDie(restConfig)

		tweakListOptionsFunc := k8s.NodeFilterTweakListOptionsFunc(nodeName)
		informer := k8s.NewStartedFilterdPodInformer(client, tweakListOptionsFunc, stopCh)

		eventBroadcaster := record.NewBroadcaster()
		eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
		eventBroadcaster.StartLogging(klog.Infof)
		recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "pod-limit-oom-recorder"})

		watcher, err := oom.NewWatcher(recorder, informer, stopCh)
		if err != nil {
			klog.Exit(err)
		}
		watcher.StartWatchAndRecord()
		klog.Info("pod-limit-oom-recorder stopped")
	})

	worker.Start()
	worker.Wait()
	klog.Info("shutdown has been completed gracefully!")
}
