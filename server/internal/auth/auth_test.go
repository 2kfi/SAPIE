package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected hash prefix: %q", hash)
	}
	if !VerifyPassword(hash, "correct horse") {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
	other, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if other == hash {
		t.Fatal("password hashes are salted identically")
	}
}

func TestTokens(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) < 32 {
		t.Fatalf("bad tokens: %q %q", a, b)
	}
	if HashToken(a) == a || !Equal(HashToken(a), HashToken(a)) || Equal(HashToken(a), HashToken(b)) {
		t.Fatal("token hashing or comparison failed")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })
	for i := 0; i < MaxFailures; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("blocked after %d failures", i)
		}
		l.RecordFailure("1.2.3.4")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("expected block after five failures")
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("other IP blocked")
	}
	now = now.Add(failureWindow + time.Second)
	if !l.Allow("1.2.3.4") {
		t.Fatal("IP not unblocked after window")
	}
}
