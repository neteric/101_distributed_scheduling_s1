package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/neteric/101_distributed_scheduling_s1/cmd/webhook/app"
)

func main() {
	ctx := controllerruntime.SetupSignalHandler()
	// Starting from version 0.15.0, controller-runtime expects its consumers to set a logger through log.SetLogger.
	// If SetLogger is not called within the first 30 seconds of a binaries lifetime, it will get
	// set to a NullLogSink and report an error. Here's to silence the "log.SetLogger(...) was never called; logs will not be displayed" error
	controllerruntime.SetLogger(klog.Background())
	cmd := app.NewWebhookCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
