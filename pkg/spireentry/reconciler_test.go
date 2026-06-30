package spireentry

import (
	"context"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"github.com/stretchr/testify/require"
)

func TestMakeEntryKey(t *testing.T) {
	id1 := spiffeid.RequireFromString("spiffe://domain.test/1")
	id2 := spiffeid.RequireFromString("spiffe://domain.test/2")
	sAABB := []spireapi.Selector{{Type: "A", Value: "A"}, {Type: "B", Value: "B"}}
	sBBAA := []spireapi.Selector{{Type: "B", Value: "B"}, {Type: "A", Value: "A"}}
	sAAAC := []spireapi.Selector{{Type: "A", Value: "A"}, {Type: "A", Value: "C"}}

	t.Run("same tuple yields same key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("selector order does not matter", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sBBAA}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("parent ID changes key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id2, SPIFFEID: id2, Selectors: sAABB}
		require.NotEqual(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("SPIFFE ID changes key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id1, Selectors: sAABB}
		require.NotEqual(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("Selectors change key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAAAC}
		require.NotEqual(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("TTL has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, X509SVIDTTL: 1}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, X509SVIDTTL: 2}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("FederatesWith has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, FederatesWith: []spiffeid.TrustDomain{spiffeid.RequireTrustDomainFromString("domaina")}}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, FederatesWith: []spiffeid.TrustDomain{spiffeid.RequireTrustDomainFromString("domainb")}}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("Admin has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Admin: false}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Admin: true}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("Downstream has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Downstream: false}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Downstream: true}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("DNSNames have no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, DNSNames: []string{"A"}}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, DNSNames: []string{"B"}}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})
}

func TestFilterJoinTokenEntries(t *testing.T) {
	id1 := spiffeid.RequireFromString("spiffe://domain.test/1")
	id2 := spiffeid.RequireFromString("spiffe://domain.test/2")
	id3 := spiffeid.RequireFromString("spiffe://domain.test/3")
	idJoinToken := spiffeid.RequireFromString("spiffe://domain.test/spire/agent/join_token/717290d1-6e81-40cc-b9c4-1416f8c30cfd")
	s1 := []spireapi.Selector{{Type: "A", Value: "A"}, {Type: "B", Value: "B"}}
	s2 := []spireapi.Selector{{Type: "B", Value: "B"}, {Type: "A", Value: "A"}}
	sJoinToken := []spireapi.Selector{{Type: "spiffe_id", Value: "A"}}

	testCases := []struct {
		name     string
		entries  []spireapi.Entry
		expected []spireapi.Entry
	}{
		{
			name: "no join token entries",
			entries: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
			},
			expected: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
			},
		},
		{
			name: "with join token entries",
			entries: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
				{ID: "3", ParentID: idJoinToken, SPIFFEID: id3, Selectors: sJoinToken},
				{ID: "4", ParentID: idJoinToken, SPIFFEID: id2, Selectors: sJoinToken},
			},
			expected: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := filterJoinTokenEntries(tc.entries)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestClusterStaticEntryPatternExpansionGate(t *testing.T) {
	// Regression guard: by default federatesWith does not deduplicate values.
	// Deduplication must only occur when EnableFederatesWithPatternExpansion is true.
	staticEntry := &ClusterStaticEntry{
		ClusterStaticEntry: spirev1alpha1.ClusterStaticEntry{
			Spec: spirev1alpha1.ClusterStaticEntrySpec{
				ParentID:      spiffeid.RequireFromString("spiffe://domain.test/1").String(),
				SPIFFEID:      spiffeid.RequireFromString("spiffe://domain.test/2").String(),
				FederatesWith: []string{"fed-a.example.org", "fed-a.example.org"},
			},
		},
	}

	t.Run("disabled preserves duplicates", func(t *testing.T) {
		r := &entryReconciler{
			config: ReconcilerConfig{
				EnableFederatesWithPatternExpansion: false,
			},
		}
		state := make(entriesState)
		r.addClusterStaticEntryEntriesState(context.Background(), state, []*ClusterStaticEntry{staticEntry}, nil)
		require.Equal(t, []string{"fed-a.example.org", "fed-a.example.org"}, federatesWithFromState(t, state))
	})

	t.Run("enabled deduplicates", func(t *testing.T) {
		r := &entryReconciler{
			config: ReconcilerConfig{
				EnableFederatesWithPatternExpansion: true,
			},
		}
		state := make(entriesState)
		r.addClusterStaticEntryEntriesState(context.Background(), state, []*ClusterStaticEntry{staticEntry}, nil)
		require.Equal(t, []string{"fed-a.example.org"}, federatesWithFromState(t, state))
	})
}

func federatesWithFromState(t *testing.T, state entriesState) []string {
	t.Helper()
	require.Len(t, state, 1)
	for _, s := range state {
		require.Len(t, s.Declared, 1)
		out := make([]string, 0, len(s.Declared[0].Entry.FederatesWith))
		for _, td := range s.Declared[0].Entry.FederatesWith {
			out = append(out, td.Name())
		}
		return out
	}
	return nil
}
