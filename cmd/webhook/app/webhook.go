package app

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/VTeam/k8s-webhook-template/cmd/webhook/app/options"
	"github.com/VTeam/k8s-webhook-template/pkg/sharedcli"
	"github.com/VTeam/k8s-webhook-template/pkg/sharedcli/klogflag"
	"github.com/VTeam/k8s-webhook-template/pkg/sharedcli/profileflag"
	gschema "github.com/VTeam/k8s-webhook-template/pkg/util/schema"
	"github.com/VTeam/k8s-webhook-template/pkg/version"
	"github.com/VTeam/k8s-webhook-template/pkg/version/sharedcommand"
	pod "github.com/VTeam/k8s-webhook-template/pkg/webhook/pod"
)

// NewWebhookCommand creates a *cobra.Command object with default parameters
func NewWebhookCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "k8s-webhook",
		Long: `The k8s-webhook starts a webhook server and manages policies about how to mutate and validate
k8s resources`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := Run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}
	setupFlag(cmd, opts)

	return cmd
}

// Run runs the webhook server with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("k8s-admission-webhook version: %s", version.Get())

	profileflag.ListenAndServe(opts.ProfileOpts)

	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

	hookManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger: klog.Background(),
		Scheme: gschema.NewSchema(),
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:     opts.BindAddress,
			Port:     opts.SecurePort,
			CertDir:  opts.CertDir,
			CertName: opts.CertName,
			KeyName:  opts.KeyName,
			TLSOpts: []func(*tls.Config){
				func(config *tls.Config) {
					// Just transform the valid options as opts.TLSMinVersion
					// can only accept "1.0", "1.1", "1.2", "1.3" and has default
					// value,
					switch opts.TLSMinVersion {
					case "1.0":
						config.MinVersion = tls.VersionTLS10
					case "1.1":
						config.MinVersion = tls.VersionTLS11
					case "1.2":
						config.MinVersion = tls.VersionTLS12
					case "1.3":
						config.MinVersion = tls.VersionTLS13
					}
				},
			},
		}),
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: opts.MetricsBindAddress},
		// HealthProbeBindAddress: opts.HealthProbeBindAddress,
	})
	if err != nil {
		klog.Errorf("Failed to build webhook server: %v", err)
		return err
	}

	decoder := admission.NewDecoder(hookManager.GetScheme())

	klog.Info("Registering webhooks to the webhook server")
	hookServer := hookManager.GetWebhookServer()

	// register validate admission webhook
	hookServer.Register("/validate-pod", &webhook.Admission{
		Handler: &pod.ValidatingAdmission{Decoder: decoder},
	})
	// register mutating admission webhook
	hookServer.Register("/mutate-pod", &webhook.Admission{
		Handler: &pod.MutatingAdmission{Decoder: decoder},
	})

	hookServer.WebhookMux().Handle("/readyz/", http.StripPrefix("/readyz/", &healthz.Handler{}))
	// hookManager.AddHealthzCheck("xxx", healthz.CheckHandler{})
	// blocks until the context is done.
	if err := hookManager.Start(ctx); err != nil {
		klog.Errorf("webhook server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

func setupFlag(cmd *cobra.Command, opts *options.Options) {
	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(genericFlagSet)

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.AddCommand(sharedcommand.NewCmdVersion("k8s-webhook"))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
}
