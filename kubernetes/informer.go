package k8s

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1informer "k8s.io/client-go/informers/core/v1"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	UIDIndex = "uid"
)

func MetaUIDIndexFunc(obj interface{}) ([]string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return []string{""}, fmt.Errorf("object has no meta: %v", err)
	}
	return []string{string(meta.GetUID())}, nil
}

func NodeFilterTweakListOptionsFunc(nodeName string) internalinterfaces.TweakListOptionsFunc {
	if nodeName == "" {
		return nil
	}

	sel := fields.ParseSelectorOrDie("spec.nodeName=" + nodeName)
	return func(options *v1.ListOptions) {
		options.FieldSelector = sel.String()
	}
}

func NewStartedFilterdPodInformer(client kubernetes.Interface, tweakListOptionsFunc internalinterfaces.TweakListOptionsFunc, stopCh chan struct{}) cache.SharedIndexInformer {
	informer := corev1informer.NewFilteredPodInformer(client, "", 0*time.Second, cache.Indexers{UIDIndex: MetaUIDIndexFunc}, tweakListOptionsFunc)

	go informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		klog.Fatal("coudn't start informer")
	}

	return informer
}
