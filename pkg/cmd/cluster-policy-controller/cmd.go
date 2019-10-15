package cluster_policy_controller

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
)

const RecommendedStartControllerName = "cluster-policy-controller"

type ClusterPolicyController struct {
	ConfigFilePath string
	// KubeConfigFile points to a kubeconfig file if you don't want to use the in cluster config
	KubeConfigFile string
	// ClientCAFile points to file if you don't use the default
	ClientCAFile string
	// TLSCertFile and TLSPrivateKeyFile point files with TLS cert,key if you don't use the default /var/run/secrets/serving-cert/
	TLSCertFile       string
	TLSPrivateKeyFile string
	Output            io.Writer
}

var longDescription = templates.LongDesc(`
	Start the Cluster Policy Controller`)

func NewClusterPolicyControllerCommand(name string, out, errout io.Writer) *cobra.Command {
	options := &ClusterPolicyController{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the cluster policy controller",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			serviceability.StartProfiler()

			if err := options.StartClusterPolicyController(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				klog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()
	// This command only supports reading from config
	flags.StringVar(&options.ConfigFilePath, "config", options.ConfigFilePath, "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	flags.StringVar(&options.KubeConfigFile, "kubeconfig", options.KubeConfigFile, "Location of the master configuration file to run from.")
	flags.StringVar(&options.ClientCAFile, "client-ca-file", options.ClientCAFile, "Location of the client-ca file to run from.")
	flags.StringVar(&options.TLSCertFile, "tls-cert-file", options.TLSCertFile, "Location of the tls cert file to run from.")
	flags.StringVar(&options.TLSPrivateKeyFile, "tls-private-key-file", options.TLSPrivateKeyFile, "Location of the tls key file to run from.")
	cmd.MarkFlagFilename("kubeconfig", "kubeconfig")
	cmd.MarkFlagFilename("client-ca-file", "client-ca-file")
	cmd.MarkFlagFilename("tls-cert-file", "tls-cert-file")
	cmd.MarkFlagFilename("tls-private-key-file", "tls-private-key-file")

	return cmd
}

// StartClusterPolicyController calls RunPolicyController and then waits forever
func (o *ClusterPolicyController) StartClusterPolicyController() error {
	if err := o.RunPolicyController(); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}

// RunPolicyController takes the options and starts the controller
func (o *ClusterPolicyController) RunPolicyController() error {

	config := &openshiftcontrolplanev1.OpenShiftControllerManagerConfig{
		/// this isn't allowed to be nil when by itself.
		ServingInfo: &configv1.HTTPServingInfo{},
	}

	if len(o.ConfigFilePath) != 0 {
		// try to decode into our new types first.  right now there is no validation, no file path resolution.  this unsticks the operator to start.
		// TODO add those things
		configContent, err := ioutil.ReadFile(o.ConfigFilePath)
		if err != nil {
			return err
		}
		scheme := runtime.NewScheme()
		utilruntime.Must(openshiftcontrolplanev1.Install(scheme))
		codecs := serializer.NewCodecFactory(scheme)
		obj, err := runtime.Decode(codecs.UniversalDecoder(openshiftcontrolplanev1.GroupVersion, configv1.GroupVersion), configContent)
		if err != nil {
			return err
		}

		// Resolve relative to CWD
		absoluteConfigFile, err := api.MakeAbs(o.ConfigFilePath, "")
		if err != nil {
			return err
		}
		configFileLocation := path.Dir(absoluteConfigFile)

		config = obj.(*openshiftcontrolplanev1.OpenShiftControllerManagerConfig)
		/// this isn't allowed to be nil when by itself.
		// TODO remove this when the old path is gone.
		if config.ServingInfo == nil {
			config.ServingInfo = &configv1.HTTPServingInfo{}
		}
		if err := helpers.ResolvePaths(getOpenShiftControllerConfigFileReferences(config), configFileLocation); err != nil {
			return err
		}
	}

	setRecommendedOpenShiftControllerConfigDefaults(config)
	if len(o.TLSCertFile) != 0 {
		config.ServingInfo.ServingInfo.CertInfo.CertFile = o.TLSCertFile
	}
	if len(o.TLSPrivateKeyFile) != 0 {
		config.ServingInfo.ServingInfo.CertInfo.KeyFile = o.TLSPrivateKeyFile
	}
	if len(o.ClientCAFile) != 0 {
		config.ServingInfo.ServingInfo.ClientCA = o.ClientCAFile
	}

	clientConfig, err := helpers.GetKubeConfigOrInClusterConfig(o.ConfigFilePath, config.KubeClientConfig.ConnectionOverrides)
	if err != nil {
		return err
	}
	return RunClusterPolicyController(config, clientConfig)
}
