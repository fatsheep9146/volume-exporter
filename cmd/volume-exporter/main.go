package main

import (
	"fmt"
	"os"

	"k8s.io/klog"

	"github.com/kpaas-io/volume-exporter/cmd/volume-exporter/app"
)

func main() {
	klog.InitFlags(nil)

	cmd := app.NewExporterCommand()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
