package controller

import (
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/volume"
)

type VolumeStats struct {
	FsStats
	Name      string
	PVCName   string
	Namespace string
}

// FsStats contains data about filesystem usage.
type FsStats struct {
	// The time at which these stats were updated.
	Time metav1.Time `json:"time"`
	// AvailableBytes represents the storage space available (bytes) for the filesystem.
	// +optional
	AvailableBytes *uint64 `json:"availableBytes,omitempty"`
	// CapacityBytes represents the total capacity (bytes) of the filesystems underlying storage.
	// +optional
	CapacityBytes *uint64 `json:"capacityBytes,omitempty"`
	// UsedBytes represents the bytes used for a specific task on the filesystem.
	// This may differ from the total bytes used on the filesystem and may not equal CapacityBytes - AvailableBytes.
	// e.g. For ContainerStats.Rootfs this is the bytes used by the container rootfs on the filesystem.
	// +optional
	UsedBytes *uint64 `json:"usedBytes,omitempty"`
	// InodesFree represents the free inodes in the filesystem.
	// +optional
	InodesFree *uint64 `json:"inodesFree,omitempty"`
	// Inodes represents the total inodes in the filesystem.
	// +optional
	Inodes *uint64 `json:"inodes,omitempty"`
	// InodesUsed represents the inodes used by the filesystem
	// This may not equal Inodes - InodesFree because this filesystem may share inodes with other "filesystems"
	// e.g. For ContainerStats.Rootfs, this is the inodes used only by that container, and does not count inodes used by other containers.
	InodesUsed *uint64 `json:"inodesUsed,omitempty"`
}

type volumesMetricProvider struct {
	pod       *v1.Pod
	providers map[string]volume.MetricsProvider
}

type volumeStatCalculator struct {
	provider     *volumesMetricProvider
	jitterPeriod time.Duration
	pod          *v1.Pod
	stopChannel  chan struct{}
	startO       sync.Once
	stopO        sync.Once
	latest       atomic.Value
}

func newVolumesMetricProvider(cli *kubernetes.Clientset, pod *v1.Pod) (*volumesMetricProvider, error) {
	providers := make(map[string]volume.MetricsProvider)
	for _, vol := range pod.Spec.Volumes {
		if claim := vol.VolumeSource.PersistentVolumeClaim; claim != nil {
			klog.Infof("new pvc %s found for pod %s/%s", claim.ClaimName, pod.Namespace, pod.Name)
			pvc, err := cli.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(claim.ClaimName, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("get pvc info from apiserver failed, err: %v", err)
			}
			providers[pvc.Name] = volume.NewMetricsStatFS("")
		}
	}

	p := &volumesMetricProvider{
		pod:       pod,
		providers: providers,
	}
	return p, nil
}

func newVolumeStatCalculator(provider *volumesMetricProvider, jitterPeriod time.Duration, pod *v1.Pod) *volumeStatCalculator {

	return &volumeStatCalculator{
		provider:     provider,
		jitterPeriod: jitterPeriod,
		pod:          pod,
		stopChannel:  make(chan struct{}),
	}
}

// StartOnce starts pod volume calc that will occur periodically in the background until s.StopOnce is called
func (s *volumeStatCalculator) StartOnce() *volumeStatCalculator {
	s.startO.Do(func() {
		go wait.JitterUntil(func() {
			s.calcAndStoreStats()
		}, s.jitterPeriod, 1.0, true, s.stopChannel)
	})
	return s
}

// StopOnce stops background pod volume calculation.  Will not stop a currently executing calculations until
// they complete their current iteration.
func (s *volumeStatCalculator) StopOnce() *volumeStatCalculator {
	s.stopO.Do(func() {
		close(s.stopChannel)
	})
	return s
}

// getLatest returns the most recent PodVolumeStats from the cache
func (s *volumeStatCalculator) GetLatest() ([]VolumeStats, bool) {
	if result := s.latest.Load(); result == nil {
		return []VolumeStats{}, false
	} else {
		return result.([]VolumeStats), true
	}
}

// calcAndStoreStats calculates PodVolumeStats for a given pod and writes the result to the s.latest cache.
// If the pod references PVCs, the prometheus metrics for those are updated with the result.
func (s *volumeStatCalculator) calcAndStoreStats() {

	// Call GetMetrics on each Volume and copy the result to a new VolumeStats.FsStats
	volumesStats := make([]VolumeStats, 0)
	for pvcname, provider := range s.provider.providers {
		metric, err := provider.GetMetrics()

		volumeStats := s.parsePodVolumeStats(s.pod.Name, pvcname, s.pod.Namespace, metric)
		volumesStats = append(volumesStats, volumeStats)
	}

	// Store the new stats
	s.latest.Store(volumesStats)
}

// parsePodVolumeStats converts (internal) volume.Metrics to (external) stats.VolumeStats structures
func (s *volumeStatCalculator) parsePodVolumeStats(podName string, pvcName string, namespace string, metric *volume.Metrics) VolumeStats {
	available := uint64(metric.Available.Value())
	capacity := uint64(metric.Capacity.Value())
	used := uint64(metric.Used.Value())
	inodes := uint64(metric.Inodes.Value())
	inodesFree := uint64(metric.InodesFree.Value())
	inodesUsed := uint64(metric.InodesUsed.Value())

	return VolumeStats{
		Name:      podName,
		PVCName:   pvcName,
		Namespace: namespace,
		FsStats: FsStats{Time: metric.Time, AvailableBytes: &available, CapacityBytes: &capacity,
			UsedBytes: &used, Inodes: &inodes, InodesFree: &inodesFree, InodesUsed: &inodesUsed},
	}
}
