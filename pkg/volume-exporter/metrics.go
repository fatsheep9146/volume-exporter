/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

const (
	KubeletSubsystem             = "kubelet"
	VolumeStatsCapacityBytesKey  = "volume_stats_capacity_bytes"
	VolumeStatsAvailableBytesKey = "volume_stats_available_bytes"
	VolumeStatsUsedBytesKey      = "volume_stats_used_bytes"
	VolumeStatsInodesKey         = "volume_stats_inodes"
	VolumeStatsInodesFreeKey     = "volume_stats_inodes_free"
	VolumeStatsInodesUsedKey     = "volume_stats_inodes_used"
)

var (
	volumeStatsCapacityBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("", KubeletSubsystem, VolumeStatsCapacityBytesKey),
		"Capacity in bytes of the volume",
		[]string{"namespace", "persistentvolumeclaim"}, nil,
	)
	volumeStatsAvailableBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("", KubeletSubsystem, VolumeStatsAvailableBytesKey),
		"Number of available bytes in the volume",
		[]string{"namespace", "persistentvolumeclaim"}, nil,
	)
	volumeStatsUsedBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("", KubeletSubsystem, VolumeStatsUsedBytesKey),
		"Number of used bytes in the volume",
		[]string{"namespace", "persistentvolumeclaim"}, nil,
	)
	volumeStatsInodesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("", KubeletSubsystem, VolumeStatsInodesKey),
		"Maximum number of inodes in the volume",
		[]string{"namespace", "persistentvolumeclaim"}, nil,
	)
	volumeStatsInodesFreeDesc = prometheus.NewDesc(
		prometheus.BuildFQName("", KubeletSubsystem, VolumeStatsInodesFreeKey),
		"Number of free inodes in the volume",
		[]string{"namespace", "persistentvolumeclaim"}, nil,
	)
	volumeStatsInodesUsedDesc = prometheus.NewDesc(
		prometheus.BuildFQName("", KubeletSubsystem, VolumeStatsInodesUsedKey),
		"Number of used inodes in the volume",
		[]string{"namespace", "persistentvolumeclaim"}, nil,
	)
)

type volumeStatsCollector struct {
	c *VolumeController
}

// NewVolumeStatsCollector creates a volume stats prometheus collector.
func NewVolumeStatsCollector(c *VolumeController) prometheus.Collector {
	return &volumeStatsCollector{c: c}
}

// Describe implements the prometheus.Collector interface.
func (collector *volumeStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- volumeStatsCapacityBytesDesc
	ch <- volumeStatsAvailableBytesDesc
	ch <- volumeStatsUsedBytesDesc
	ch <- volumeStatsInodesDesc
	ch <- volumeStatsInodesFreeDesc
	ch <- volumeStatsInodesUsedDesc
}

// Collect implements the prometheus.Collector interface.
func (collector *volumeStatsCollector) Collect(ch chan<- prometheus.Metric) {

	addGauge := func(desc *prometheus.Desc, pvcname, namespace string, v float64, lv ...string) {
		lv = append([]string{namespace, pvcname}, lv...)
		metric, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, v, lv...)
		if err != nil {
			klog.Warningf("Failed to generate metric: %v", err)
			return
		}
		ch <- metric
	}
	allPVCs := sets.String{}
	for _, vc := range collector.c.podToVolumes {
		volumeStats, _ := vc.GetLatest()
		for _, vs := range volumeStats {
			addGauge(volumeStatsCapacityBytesDesc, vs.PVCName, vs.Namespace, float64(*vs.CapacityBytes))
			addGauge(volumeStatsAvailableBytesDesc, vs.PVCName, vs.Namespace, float64(*vs.AvailableBytes))
			addGauge(volumeStatsUsedBytesDesc, vs.PVCName, vs.Namespace, float64(*vs.UsedBytes))
			addGauge(volumeStatsInodesDesc, vs.PVCName, vs.Namespace, float64(*vs.Inodes))
			addGauge(volumeStatsInodesFreeDesc, vs.PVCName, vs.Namespace, float64(*vs.InodesFree))
			addGauge(volumeStatsInodesUsedDesc, vs.PVCName, vs.Namespace, float64(*vs.InodesUsed))
		}
	}
}
