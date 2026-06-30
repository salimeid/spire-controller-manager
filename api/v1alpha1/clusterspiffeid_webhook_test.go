package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseClusterSPIFFEIDSpecRejectsPatternsByDefault(t *testing.T) {
	spec := &ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		FederatesWith: []string{
			"fed-*.example.org",
		},
	}

	parsed, err := ParseClusterSPIFFEIDSpec(spec)
	require.Error(t, err)
	require.Nil(t, parsed)
}

func TestParseClusterSPIFFEIDSpecWithPatternExpansionAllowsPatterns(t *testing.T) {
	spec := &ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		FederatesWith: []string{
			"fed-*.example.org",
			"fed-?.example.org",
			"fed-[ab].example.org",
			"fed-static.example.org",
		},
	}

	parsed, err := ParseClusterSPIFFEIDSpecWithPatternExpansion(spec)
	require.NoError(t, err)
	require.Len(t, parsed.FederatesWith, 1)
	require.Equal(t, "fed-static.example.org", parsed.FederatesWith[0].Name())
}
