package controller

import (
	"fmt"
	"sync"
	"time"

	// coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	// "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	// "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type VolumeController struct {
	cli       *kubernetes.Clientset
	podLister corelister.PodLister
	podSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	podToVolumes map[string]*volumeStatCalculator
	// volumeToPodIDs map[types.UID]sets.String
	lock sync.Mutex
}

func NewVolumeController(
	cli *kubernetes.Clientset,
	podInformer cache.SharedIndexInformer,
) (*VolumeController, error) {
	vc := &VolumeController{
		cli:          cli,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Pods"),
		podToVolumes: make(map[string]*volumeStatCalculator),
		// volumeToPodIDs: make(map[types.UID]sets.String),
	}

	vc.podLister = corelister.NewPodLister(podInformer.GetIndexer())
	vc.podSynced = podInformer.HasSynced

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    vc.add,
		UpdateFunc: vc.update,
		DeleteFunc: vc.delete,
	})

	return vc, nil
}

func (c *VolumeController) Run(stop <-chan struct{}) error {
	defer c.queue.ShutDown()

	klog.Infof("starting volume controller")

	klog.Infof("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stop, c.podSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Infof("starting workers")
	for i := 0; i < 2; i++ {

	}
	klog.Infof("started workers")
	return nil
}

func (c *VolumeController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *VolumeController) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.queue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *VolumeController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	isDeletion := false
	pod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// utilruntime.HandleError(fmt.Errorf("pod '%s' in work queue no longer exists", key))
			klog.Infof("pod %s/%s is deleted", namespace, name)
			isDeletion = true
		} else {
			return err
		}
	}

	if isDeletion {
		if _, ok := c.podToVolumes[key]; ok {
			klog.Infof("delete pod %s/%s from volume controller", namespace, name)
			if err = c.deletePod(key); err != nil {
				klog.Errorf("delete pod %s/%s from volume controller failed, err: %v", namespace, name, err)
				return err
			}
			klog.Infof("delete pod %s/%s from volume controller succeeded", namespace, name)
		}
		return nil
	}

	err = c.addPod(pod, key)
	if err != nil {
		klog.Errorf("add pod %s/%s into volume controller failed, err: %v", err)
	}

	return nil
}

func (c *VolumeController) addPod(pod *v1.Pod, key string) error {
	if c.podExists(key) {
		return nil
	}

	provider, err := newVolumesMetricProvider(c.cli, pod)
	if err != nil {

	}

	calcultor := newVolumeStatCalculator(provider, time.Second, pod)

	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.podToVolumes[key]; !ok {
		c.podToVolumes[key] = calcultor.StartOnce()
	}

	return nil
}

func (c *VolumeController) deletePod(key string) error {

	return nil
}

func (c *VolumeController) podExists(key string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, ok := c.podToVolumes[key]; ok {
		return true
	} else {
		return false
	}
}

func (c *VolumeController) add(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		// utilruntime.HandleError(err)
		klog.Error(err)
		return
	}
	klog.Infof("[   Add  ] action: [%s]", key)
	c.queue.Add(key)
}

func (c *VolumeController) update(oldObj, newObj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(newObj); err != nil {
		// utilruntime.HandleError(err)
		klog.Error(err)
		return
	}
	klog.Infof("[ Update ] action: [%s]", key)
	c.queue.Add(key)
}

func (c *VolumeController) delete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		// utilruntime.HandleError(err)
		klog.Error(err)
		return
	}
	klog.Infof("[ Delete ] action: [%s]", key)
	c.queue.Add(key)
}
