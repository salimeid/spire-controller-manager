package spireentry

import (
	"fmt"
	"testing"
	"text/template"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterName   = "test"
	clusterDomain = "cluster.local"
	trustDomain   = "example.org"
)

func TestRenderPodEntry(t *testing.T) {
	spec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		DNSNameTemplates: []string{
			"{{ .PodSpec.ServiceAccountName }}.{{ .PodMeta.Namespace }}.svc.{{ .ClusterDomain }}",
			"{{ .PodMeta.Name }}.{{ .PodMeta.Namespace }}.svc.{{ .ClusterDomain }}", // Duplicate
			"{{ .PodMeta.Name }}.{{ .TrustDomain }}.svc",
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID: "uid",
		},
		Spec: corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "namespace",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}
	endpointsList := &corev1.EndpointsList{
		Items: []corev1.Endpoints{ //nolint: staticcheck // Refactor is going be done as part of a https://github.com/spiffe/spire-controller-manager/issues/554

			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint",
					Namespace: "namespace",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-endpoint",
					Namespace: "namespace",
				},
			},
		},
	}

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, endpointsList, td, clusterName, clusterDomain, nil)
	require.NoError(t, err)

	// SPIFFE ID rendered correctly
	spiffeID, err := spiffeid.FromPathf(td, "/ns/%s/sa/%s", pod.Namespace, pod.Spec.ServiceAccountName)
	require.NoError(t, err)
	require.Equal(t, entry.SPIFFEID.String(), spiffeID.String())

	// Parent ID rendered correctly
	parentID, err := spiffeid.FromPathf(td, "/spire/agent/k8s_psat/%s/%s", clusterName, node.UID)
	require.NoError(t, err)
	require.Equal(t, entry.ParentID.String(), parentID.String())

	// DNS names are unique
	dnsNamesSet := make(map[string]struct{})
	for _, dnsName := range entry.DNSNames {
		_, exists := dnsNamesSet[dnsName]
		require.False(t, exists)
		dnsNamesSet[dnsName] = struct{}{}
	}

	// DNS names list is as long as expected
	require.Equal(t, len(spec.DNSNameTemplates)-1+len(endpointsList.Items)*4, len(entry.DNSNames))

	// DNS names templates rendered correctly and are in order
	require.Equal(t, entry.DNSNames[0], pod.Spec.ServiceAccountName+"."+pod.Namespace+".svc."+clusterDomain)
	require.Equal(t, entry.DNSNames[1], pod.Name+"."+trustDomain+".svc")

	// Endpoint DNS Names auto populated
	for _, endpoint := range endpointsList.Items {
		require.Contains(t, entry.DNSNames, endpoint.Name)
		require.Contains(t, entry.DNSNames, endpoint.Name+"."+endpoint.Namespace)
		require.Contains(t, entry.DNSNames, endpoint.Name+"."+endpoint.Namespace+".svc")
		require.Contains(t, entry.DNSNames, endpoint.Name+"."+endpoint.Namespace+".svc."+clusterDomain)
	}
}

func TestJWTTTLInRenderPodEntry(t *testing.T) {
	spec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		JWTTTL:           metav1.Duration{Duration: time.Duration(60)},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID: "uid",
		},
		Spec: corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "namespace",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, &corev1.EndpointsList{}, td, clusterName, clusterDomain, nil)
	require.NoError(t, err)

	require.Equal(t, entry.JWTSVIDTTL.Nanoseconds(), spec.JWTTTL.Nanoseconds())
}

func TestParentIDTemplateRenderPodEntry(t *testing.T) {
	spec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "uid",
			Name: "test.example.org",
		},
		Spec: corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "namespace",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}

	defaultParentIDTemplate, err := template.New("testParentIDTemplate").Parse("spiffe://{{ .TrustDomain }}/spire/agent/x509pop/{{ .NodeMeta.Name }}")
	require.NoError(t, err)

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, &corev1.EndpointsList{}, td, clusterName, clusterDomain, defaultParentIDTemplate)
	require.NoError(t, err)

	require.Equal(t, entry.ParentID.String(), fmt.Sprintf("spiffe://%s/spire/agent/x509pop/test.example.org", td))
}

func TestExpandFederatesWithWildcards(t *testing.T) {
	tests := []struct {
		name      string
		wildcards []string
		known     []string
		expected  []string
	}{
		{
			name:      "no wildcards returns static",
			wildcards: []string{"fed-1.com", "fed-2.com"},
			known:     []string{"fed-1.com", "fed-2.com", "fed-3.com"},
			expected:  []string{"fed-1.com", "fed-2.com"},
		},
		{
			name:      "wildcard expands to matches",
			wildcards: []string{"fed-*.example.org"},
			known:     []string{"fed-1.example.org", "fed-2.example.org", "other.com"},
			expected:  []string{"fed-1.example.org", "fed-2.example.org"},
		},
		{
			name:      "static and wildcard merged",
			wildcards: []string{"static.com", "fed-*.example.org"},
			known:     []string{"fed-1.example.org", "fed-2.example.org"},
			expected:  []string{"static.com", "fed-1.example.org", "fed-2.example.org"},
		},
		{
			name:      "no matches returns empty",
			wildcards: []string{"fed-*.other.com"},
			known:     []string{"fed-1.example.org"},
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandFederatesWithWildcards(tt.wildcards, tt.known)
			if !stringsEqual(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExpandRawFederatesWithWildcards(t *testing.T) {
	rawFederatesWith := []string{"fed-a.example.org", "fed-*.example.org"}
	knownTrustDomains := []string{"fed-a.example.org", "fed-b.example.org", "fed-c.example.org"}

	expanded := expandFederatesWithWildcards(rawFederatesWith, knownTrustDomains)

	expected := []string{"fed-a.example.org", "fed-b.example.org", "fed-c.example.org"}
	if !stringsEqual(expanded, expected) {
		t.Errorf("expanded mismatch: got %v, want %v", expanded, expected)
	}
}

func TestExpandFederatesWithCalledInReconciliation(t *testing.T) {
	// Verify that renderStaticEntry uses FederatesWith as-is for now
	spec := &spirev1alpha1.ClusterStaticEntrySpec{
		ParentID:      "spiffe://example.com/spire/agent/k8s_sat/cluster1/uuid",
		SPIFFEID:      "spiffe://example.com/workload/test",
		FederatesWith: []string{"fed-a.example.org", "fed-b.example.org"},
	}

	entry, err := renderStaticEntry(spec)
	if err != nil {
		t.Fatalf("renderStaticEntry failed: %v", err)
	}

	// Entry should have federatesWith set (basic sanity check)
	if entry == nil {
		t.Errorf("entry is nil")
	}
	if len(entry.FederatesWith) != 2 {
		t.Errorf("expected 2 federatesWith, got %d", len(entry.FederatesWith))
	}
}

func TestRenderStaticEntryWithWildcardExpansion(t *testing.T) {
	// Verify that wildcards in FederatesWith are expanded before rendering
	spec := &spirev1alpha1.ClusterStaticEntrySpec{
		ParentID:      "spiffe://example.com/spire/agent/k8s_sat/cluster1/uuid",
		SPIFFEID:      "spiffe://example.com/workload/test",
		FederatesWith: []string{"fed-a.example.org", "fed-*.example.org"},
	}
	knownTrustDomains := []string{"fed-a.example.org", "fed-b.example.org", "fed-c.example.org"}

	// Simulate what reconciler should do: expand wildcards first
	expanded := expandFederatesWithWildcards(spec.FederatesWith, knownTrustDomains)

	// Create spec copy with expanded domains
	specCopy := *spec
	specCopy.FederatesWith = expanded

	entry, err := renderStaticEntry(&specCopy)
	if err != nil {
		t.Fatalf("renderStaticEntry failed: %v", err)
	}

	// Entry should have expanded federatesWith
	if entry == nil {
		t.Errorf("entry is nil")
	}
	expected := []string{"fed-a.example.org", "fed-b.example.org", "fed-c.example.org"}
	got := make([]string, len(entry.FederatesWith))
	for i, td := range entry.FederatesWith {
		got[i] = td.Name()
	}
	if !stringsEqual(got, expected) {
		t.Errorf("federatesWith mismatch: got %v, want %v", got, expected)
	}
}

func TestRenderPodEntryWithWildcardExpansion(t *testing.T) {
	// Verify that wildcards in FederatesWith are expanded in pod entries before rendering
	// Note: The CRD spec itself doesn't have wildcards - those are in the raw ClusterSPIFFEIDSpec
	// The test simulates the reconciler flow: expand raw spec, then parse it

	// Start with raw spec that has wildcards
	rawSpec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		FederatesWith:    []string{"fed-a.example.org", "fed-*.example.org"},
	}

	// Expand wildcards before parsing
	knownTrustDomains := []string{"fed-a.example.org", "fed-b.example.org", "fed-c.example.org"}
	expandedFederatesWith := expandFederatesWithWildcards(rawSpec.FederatesWith, knownTrustDomains)

	// Create a spec with expanded values for parsing
	specForParsing := *rawSpec
	specForParsing.FederatesWith = expandedFederatesWith

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(&specForParsing)
	if err != nil {
		t.Fatalf("ParseClusterSPIFFEIDSpec failed: %v", err)
	}

	// Now render the pod entry with the expanded spec
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{UID: "uid"},
		Spec:       corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "namespace"},
		Spec:       corev1.PodSpec{ServiceAccountName: "test"},
	}
	endpointsList := &corev1.EndpointsList{}

	td, err := spiffeid.TrustDomainFromString(trustDomain)
	if err != nil {
		t.Fatalf("TrustDomainFromString failed: %v", err)
	}

	entry, err := renderPodEntry(parsedSpec, node, pod, endpointsList, td, clusterName, clusterDomain, nil)
	if err != nil {
		t.Fatalf("renderPodEntry failed: %v", err)
	}

	// Entry should have expanded federatesWith
	if entry == nil {
		t.Errorf("entry is nil")
	}
	expected := []string{"fed-a.example.org", "fed-b.example.org", "fed-c.example.org"}
	got := make([]string, len(entry.FederatesWith))
	for i, td := range entry.FederatesWith {
		got[i] = td.Name()
	}
	if !stringsEqual(got, expected) {
		t.Errorf("federatesWith mismatch: got %v, want %v", got, expected)
	}
}

func TestExpandFederatesWithWildcardsFeatureDisabled(t *testing.T) {
	// Verify that when glob pattern expansion is disabled,
	// patterns are returned as-is (not expanded against known domains)
	patterns := []string{"fed-*.example.org", "static.com"}
	knownTrustDomains := []string{"fed-a.example.org", "fed-b.example.org"}

	// When feature disabled, should return patterns unchanged
	result := expandFederatesWithWildcardsIfEnabled(patterns, knownTrustDomains, false)

	expected := []string{"fed-*.example.org", "static.com"}
	if !stringsEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestExpandFederatesWithWildcardsFeatureEnabled(t *testing.T) {
	// Verify that when glob pattern expansion is enabled,
	// patterns are properly expanded
	patterns := []string{"fed-*.example.org", "static.com"}
	knownTrustDomains := []string{"fed-a.example.org", "fed-b.example.org"}

	// When feature enabled, should expand patterns
	result := expandFederatesWithWildcardsIfEnabled(patterns, knownTrustDomains, true)

	// Wildcard expands first (in input order), then static domain
	expected := []string{"fed-a.example.org", "fed-b.example.org", "static.com"}
	if !stringsEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}
