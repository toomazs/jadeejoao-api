package platform

import "testing"

func TestRateLimiterBurstAndIsolation(t *testing.T) {
	l := NewRateLimiter(60, 2)

	// Burst of 2 for one IP, then denied.
	for i := 0; i < 2; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("burst request %d must pass", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("third immediate request must be denied")
	}

	// Another IP has its own bucket.
	if !l.Allow("5.6.7.8") {
		t.Fatal("distinct keys must not share buckets")
	}
}
