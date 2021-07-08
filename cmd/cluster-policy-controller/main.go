package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/project"
	"github.com/openshift/api/security"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"

	cluster_policy_controller "github.com/openshift/cluster-policy-controller/pkg/cmd/cluster-policy-controller"
)

func init() {
	// TODO: these references to the legacy scheme must go
	//  They are only here because we have controllers referencing it, and inside hypershift this worked fine as openshift-apiserver was installing the API into the legacy scheme.
	utilruntime.Must(apps.Install(legacyscheme.Scheme))
	utilruntime.Must(authorization.Install(legacyscheme.Scheme))
	utilruntime.Must(build.Install(legacyscheme.Scheme))
	utilruntime.Must(image.Install(legacyscheme.Scheme))
	utilruntime.Must(oauth.Install(legacyscheme.Scheme))
	utilruntime.Must(project.Install(legacyscheme.Scheme))
	utilruntime.Must(security.Install(legacyscheme.Scheme))
	utilruntime.Must(template.Install(legacyscheme.Scheme))
	utilruntime.Must(user.Install(legacyscheme.Scheme))
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := NewClusterPolicyControllerCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewClusterPolicyControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster-policy-controller",
		Short: "Command for the OpenShift Cluster Policy Controller",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}
	start := cluster_policy_controller.NewClusterPolicyControllerCommand("start")
	cmd.AddCommand(start)

	return cmd
}
