package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_newMonitoringCSR(t *testing.T) {
	csr, err := newMonitoringCSR("foo", "bar")
	require.NoError(t, err)
	require.Equal(t, "foo", csr.subject)
	require.Equal(t, "CN=system:serviceaccount:openshift-monitoring:bar", csr.certSubject)
	require.Equal(t, "metrics.openshift.io/csr.subject=foo", csr.selector.String())

	csr, err = newMonitoringCSR("_invalid-key", "bar")
	require.Error(t, err)
	require.Nil(t, csr)
}
