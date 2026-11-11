package controller

import (
	"context"
	"slices"

	"github.com/openshift/cluster-policy-controller/pkg/psalabelsyncer"
)

// psaLabelSyncerMode is the way the PSA label syncer runs.
type psaLabelSyncerMode string

const (
	// psaLabelSyncerEnforcing syncs the enforce, warn and audit PSA labels.
	psaLabelSyncerEnforcing psaLabelSyncerMode = "enforcing"
	// psaLabelSyncerAdvising syncs only the warn and audit PSA labels.
	psaLabelSyncerAdvising psaLabelSyncerMode = "advising"
	// psaLabelSyncerDisabled does not start the PSA label syncer.
	psaLabelSyncerDisabled psaLabelSyncerMode = "disabled"
)

// psaLabelSyncerModeFor picks the mode of the PSA label syncer from the
// feature gates in the controller config.
func psaLabelSyncerModeFor(featureGates []string) psaLabelSyncerMode {
	switch {
	case len(featureGates) == 0:
		// MicroShift passes no feature gates at all. It enforces restricted
		// by default and relies on the PSA label syncer to set enforce labels.
		return psaLabelSyncerEnforcing
	case slices.Contains(featureGates, "OpenShiftPodSecurityAdmission=false"):
		// OpenShift and HyperShift pass every feature gate, enabled or
		// disabled. With the gate disabled, they keep the warn and audit labels.
		return psaLabelSyncerAdvising
	default:
		// The gate is enabled or no longer listed. OpenShift and HyperShift
		// do not run the PSA label syncer then, so it never sets enforce
		// labels there, including during upgrades and rollbacks.
		return psaLabelSyncerDisabled
	}
}

func runPodSecurityAdmissionLabelSynchronizationController(ctx context.Context, controllerCtx *EnhancedControllerContext) (bool, error) {

	kubeClient, err := controllerCtx.ClientBuilder.Client(podSecurityAdmissionLabelSyncerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	switch psaLabelSyncerModeFor(controllerCtx.OpenshiftControllerConfig.FeatureGates) {
	case psaLabelSyncerAdvising:
		controller, err := psalabelsyncer.NewAdvisingPodSecurityAdmissionLabelSynchronizationController(
			kubeClient.CoreV1().Namespaces(),
			controllerCtx.KubernetesInformers.Core().V1().Namespaces(),
			controllerCtx.KubernetesInformers.Rbac().V1(),
			controllerCtx.KubernetesInformers.Core().V1().ServiceAccounts(),
			controllerCtx.SecurityInformers.Security().V1().SecurityContextConstraints(),
			controllerCtx.EventRecorder.ForComponent("podsecurity-admission-label-sync-controller"),
		)
		if err != nil {
			return true, err
		}
		go controller.Run(ctx, 1)

	case psaLabelSyncerEnforcing:
		controller, err := psalabelsyncer.NewEnforcingPodSecurityAdmissionLabelSynchronizationController(
			kubeClient.CoreV1().Namespaces(),
			controllerCtx.KubernetesInformers.Core().V1().Namespaces(),
			controllerCtx.KubernetesInformers.Rbac().V1(),
			controllerCtx.KubernetesInformers.Core().V1().ServiceAccounts(),
			controllerCtx.SecurityInformers.Security().V1().SecurityContextConstraints(),
			controllerCtx.EventRecorder.ForComponent("podsecurity-admission-label-sync-controller"),
		)
		if err != nil {
			return true, err
		}
		go controller.Run(ctx, 1)

	default:
		return false, nil
	}

	return true, nil
}

func runPrivilegedNamespacesPSALabelSyncer(ctx context.Context, controllerCtx *EnhancedControllerContext) (bool, error) {
	kubeClient, err := controllerCtx.ClientBuilder.Client(privilegedNamespacesPodSecurityAdmissionLabelSyncerServiceAccountName)
	if err != nil {
		return true, err
	}

	controller := psalabelsyncer.NewPrivilegedNamespacesPSALabelSyncer(
		ctx,
		kubeClient.CoreV1().Namespaces(),
		controllerCtx.KubernetesInformers.Core().V1().Namespaces(),
		controllerCtx.EventRecorder.ForComponent("privileged-namespaces-psa-label-syncer"),
	)

	go controller.Run(ctx, 1)

	return true, nil
}
