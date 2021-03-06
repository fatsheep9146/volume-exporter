package app

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	api "k8s.io/kubernetes/pkg/apis/core"

	"github.com/kpaas-io/volume-exporter/pkg/volume-exporter"
)

const (
	AllNamespace = ""
)

type VolumeExporterOption struct {
	port       int32
	kubeconfig string
}

func NewVolumeExporterOption() *VolumeExporterOption {

	return &VolumeExporterOption{
		port: 9876,
	}
}

func NewExporterCommand() *cobra.Command {
	opt := NewVolumeExporterOption()

	cmd := &cobra.Command{
		Use: "exporter",
		Run: func(cmd *cobra.Command, args []string) {
			flag.Parse()

			nodename := getHostName()

			cli, err := buildClientset(opt.kubeconfig)
			if err != nil {
				cmd.Usage()
				klog.Fatalf("build clientset failed, err %v", err)
			}

			podInformer := coreinformer.NewFilteredPodInformer(
				cli,
				AllNamespace,
				time.Second*30,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
				func(opt *v1.ListOptions) {
					selector := fields.OneTermEqualSelector(api.PodHostField, string(nodename))
					opt.FieldSelector = selector.String()
				},
			)

			c, err := controller.NewVolumeController(
				cli,
				podInformer)
			if err != nil {
				cmd.Usage()
				klog.Fatalf("new volume controller failed, err %v", err)
			}

			stop := make(chan struct{})

			go podInformer.Run(stop)

			go c.Run(stop)

			collector := controller.NewVolumeStatsCollector(c)
			prometheus.Register(collector)
			http.Handle("/metrics", promhttp.Handler())
			go func() {
				klog.Infof("starting http server, listening on :%d", opt.port)
				if err := http.ListenAndServe(fmt.Sprintf(":%d", opt.port), nil); err != nil && err != http.ErrServerClosed {
					klog.Fatalf("start http server error, err: %v", err)
				}
			}()

			<-stop
		},
	}

	flag.Int32Var(&opt.port, "port", opt.port, "the port that exporter listen to")
	flag.StringVar(&opt.kubeconfig, "kubeconfig", opt.kubeconfig, "the path of kubeconfig file")

	return cmd
}

func buildClientset(kubeconfig string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, err
	}

	cli, err := kubernetes.NewForConfig(config)
	return cli, err
}

func getHostName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}
