package route

import (
	"testing"
	"time"
)

func TestUploadRateLimiterShouldRemoveExpiredThreadBuckets(t *testing.T) {
	limiter := newUploadRateLimiter(10, time.Minute)
	start := time.Unix(0, 0)
	if !limiter.allowThread(1, "amp", "T-old", start) {
		t.Fatal("allow old thread bucket")
	}
	if !limiter.allowThread(1, "amp", "T-current", start.Add(time.Minute)) {
		t.Fatal("allow current thread bucket")
	}
	if len(limiter.keys) != 1 {
		t.Fatalf("retained thread buckets = %d, want 1", len(limiter.keys))
	}
}

func TestUploadRateLimiterShouldIsolateUserProviderAndThreadID(t *testing.T) {
	limiter := newUploadRateLimiter(10, time.Minute)
	now := time.Unix(1, 0)
	for index := 0; index < 10; index++ {
		if !limiter.allowThread(1, "amp", "T-shared", now) {
			t.Fatalf("allow same user/provider/thread at attempt %d", index+1)
		}
	}
	if limiter.allowThread(1, "amp", "T-shared", now) {
		t.Fatal("same user/provider/thread should be rate limited")
	}
	if !limiter.allowThread(2, "amp", "T-shared", now) {
		t.Fatal("another user should have an independent thread bucket")
	}
	if !limiter.allowThread(1, "claude_code", "T-shared", now) {
		t.Fatal("another provider should have an independent thread bucket")
	}
	if !limiter.allowThread(1, "amp", "T-other", now) {
		t.Fatal("another thread should have an independent bucket")
	}
}
