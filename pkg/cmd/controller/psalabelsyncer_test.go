package controller

import (
	"testing"
)

func TestPSALabelSyncerModeFor(t *testing.T) {
	tests := []struct {
		name         string
		featureGates []string
		want         psaLabelSyncerMode
	}{
		{
			name:         "no feature gates, as on MicroShift, means enforcing",
			featureGates: nil,
			want:         psaLabelSyncerEnforcing,
		},
		{
			name:         "an empty feature gate list means enforcing",
			featureGates: []string{},
			want:         psaLabelSyncerEnforcing,
		},
		{
			name:         "a disabled gate means advising",
			featureGates: []string{"AdminNetworkPolicy=true", "OpenShiftPodSecurityAdmission=false"},
			want:         psaLabelSyncerAdvising,
		},
		{
			name:         "an enabled gate means disabled",
			featureGates: []string{"AdminNetworkPolicy=true", "OpenShiftPodSecurityAdmission=true"},
			want:         psaLabelSyncerDisabled,
		},
		{
			name:         "a gate that is no longer listed means disabled",
			featureGates: []string{"AdminNetworkPolicy=true", "BuildCSIVolumes=false"},
			want:         psaLabelSyncerDisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := psaLabelSyncerModeFor(tt.featureGates); got != tt.want {
				t.Errorf("psaLabelSyncerModeFor(%v) = %q, want %q", tt.featureGates, got, tt.want)
			}
		})
	}
}
