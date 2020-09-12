package oom

import (
	"strings"
	"time"

	"github.com/google/cadvisor/utils/oomparser"
	k8s "github.com/uesyn/pod-limit-oom-recorder/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type watcher struct {
	startTime time.Time
	parser    *oomparser.OomParser
	informer  cache.SharedIndexInformer
	recorder  record.EventRecorder
	stopCh    chan struct{}
}

func NewWatcher(recorder record.EventRecorder, informer cache.SharedIndexInformer, stopCh chan struct{}) (*watcher, error) {
	startTime := time.Now()

	parser, err := oomparser.New()
	if err != nil {
		return nil, err
	}

	return &watcher{
		startTime: startTime,
		parser:    parser,
		informer:  informer,
		recorder:  recorder,
		stopCh:    stopCh,
	}, nil
}

func getPodIDFromContainerName(name string) string {
	eles := strings.Split(name, "/")
	if len(eles) < 2 {
		klog.Fatal("unsupported format")
	}
	podIDele := eles[len(eles)-2]
	podID := strings.TrimLeft(podIDele, "pod")
	if podID == "" {
		klog.Fatal("coudn't get pod ID")
	}
	return podID
}

func (w *watcher) StartWatchAndRecord() {
	oomInstStream := make(chan *oomparser.OomInstance, 10)
	go w.parser.StreamOoms(oomInstStream)

	for {
		var oomInst *oomparser.OomInstance
		select {
		case oomInst = <-oomInstStream:
		case <-w.stopCh:
			return
		}
		// record Pod OOM event after this process started
		if !oomInst.TimeOfDeath.After(w.startTime) {
			continue
		}
		podId := getPodIDFromContainerName(oomInst.ContainerName)
		podObjs, err := w.informer.GetIndexer().ByIndex(k8s.UIDIndex, podId)
		if err != nil {
			klog.Error(err)
			continue
		}
		for _, podObj := range podObjs {
			pod, _ := podObj.(*corev1.Pod)
			messageFmt := "OOMkiller killed %s"
			w.recorder.Eventf(pod, corev1.EventTypeWarning, "ContainerOOM", messageFmt, oomInst.ProcessName)
		}
	}
}
