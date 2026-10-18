package thread

import (
	"testing"
	"time"

	"github.com/jieliapp/jieli/backend/model"
)

// cleanupOrderKey is the single source of truth for auto-cleanup deletion order
// (D9): prefer the user's last interaction time, then the provider-created time,
// then the row creation time. These cases lock in the fallback chain.
func TestCleanupOrderKeyFallbackChain(t *testing.T) {
	created := time.UnixMilli(5000)

	cases := []struct {
		name string
		in   *model.Thread
		want int64
	}{
		{
			name: "uses user last interacted when present",
			in:   &model.Thread{UserLastInteractedAtMS: 100, ProviderCreatedMillis: 200},
			want: 100,
		},
		{
			name: "falls back to provider created when last interacted is zero",
			in:   &model.Thread{UserLastInteractedAtMS: 0, ProviderCreatedMillis: 200},
			want: 200,
		},
		{
			name: "falls back to row created when both are zero",
			in:   &model.Thread{UserLastInteractedAtMS: 0, ProviderCreatedMillis: 0},
			want: created.UnixMilli(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.in.ProviderCreatedMillis == 0 && tc.in.UserLastInteractedAtMS == 0 {
				tc.in.CreatedAt = created
			}
			if got := cleanupOrderKey(tc.in); got != tc.want {
				t.Fatalf("cleanupOrderKey = %d, want %d", got, tc.want)
			}
		})
	}
}

// Ordering must be transitive across the fallback boundaries: a thread keyed by
// ProviderCreatedMillis can still sort older or newer than one keyed by
// UserLastInteractedAtMS.
func TestCleanupOrderKeyComparesAcrossFallbacks(t *testing.T) {
	older := &model.Thread{UserLastInteractedAtMS: 0, ProviderCreatedMillis: 50}
	newer := &model.Thread{UserLastInteractedAtMS: 100}
	if cleanupOrderKey(older) >= cleanupOrderKey(newer) {
		t.Fatalf("expected provider-created 50 to sort before last-interacted 100")
	}
}
