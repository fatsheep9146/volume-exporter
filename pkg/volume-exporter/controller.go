package controller

import (
	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

type VolumeController struct {
	cli       *kubernetes.Clientset
	podLister *corelister.podLister
	podSynced cache.InformerSynced
}

func NewVolumeController(
	cli *kubernetes.Clientset,
	podInformer *cache.SharedIndexInformer,
) (*VolumeController, error) {
	vc := &VolumeController{
		cli: cli,
	}

	vc.podLister = corelister.NewPodLister(podInformer.GetIndexer())
	vc.podSynced = podInformer.HasSynced

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    add,
		UpdateFunc: update,
		DeleteFunc: delete,
	})

	return vc, nil
}

func (c *VolumeController) Run(stop chan<- struct{}) error {

	return nil
}

func add(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		// utilruntime.HandleError(err)
		klog.Error(err)
		return
	}
	klog.Infof("[   Add  ] action: [%s]", key)
}

func update(oldObj, newObj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(newObj); err != nil {
		// utilruntime.HandleError(err)
		klog.Error(err)
		return
	}
	klog.Infof("[ Update ] action: [%s]", key)
}

func delete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		// utilruntime.HandleError(err)
		klog.Error(err)
		return
	}
	klog.Infof("[ Delete ] action: [%s]", key)
}
