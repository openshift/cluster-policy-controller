package cluster_policy_controller

import (
	"io"

	"github.com/spf13/cobra"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/cluster-policy-controller/pkg/version"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

var longDescription = templates.LongDesc(`
	Start the Cluster Policy Controller`)

func NewClusterPolicyControllerCommand(name string, out, errout io.Writer) *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("cluster-policy-controller", version.Get(), RunPolicyController).
		NewCommand()
	cmd.Use = "start"
	cmd.Short = "Start the cluster policy controller"

	return cmd
}

// RunPolicyController takes the options and starts the controller
func RunPolicyController(ctx *controllercmd.ControllerContext) error {
	config := &openshiftcontrolplanev1.OpenShiftControllerManagerConfig{
		ServingInfo: &configv1.HTTPServingInfo{},
	}
	setRecommendedOpenShiftControllerConfigDefaults(config)

	return RunClusterPolicyController(config, ctx.KubeConfig)
}
