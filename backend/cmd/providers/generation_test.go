package providers

import (
	"testing"
	"time"
)

func TestGeneration_CancelStopsContext(t *testing.T) {
	const msgID = 42
	const user = "alice"

	ctx := StartGeneration(msgID, user)
	defer EndGeneration(msgID)

	if IsGenerationCancelled(msgID) {
		t.Fatal("expected generation not cancelled initially")
	}

	if !CancelStream(msgID, user) {
		t.Fatal("expected CancelStream to succeed")
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected generation context to be cancelled")
	}

	if !IsGenerationCancelled(msgID) {
		t.Fatal("expected IsGenerationCancelled after cancel")
	}
}

func TestGeneration_CancelRejectsWrongUser(t *testing.T) {
	const msgID = 43
	ctx := StartGeneration(msgID, "alice")
	defer EndGeneration(msgID)

	if CancelStream(msgID, "bob") {
		t.Fatal("expected CancelStream to reject wrong user")
	}
	if ctx.Err() != nil {
		t.Fatal("expected generation context still active for wrong-user cancel")
	}
}

func TestGeneration_EndCleansUp(t *testing.T) {
	const msgID = 44
	StartGeneration(msgID, "alice")
	EndGeneration(msgID)

	if CancelStream(msgID, "alice") {
		t.Fatal("expected CancelStream to fail after EndGeneration")
	}
	if IsGenerationCancelled(msgID) {
		t.Fatal("expected no cancelled generation after cleanup")
	}
}

func TestGeneration_ReplacePrevious(t *testing.T) {
	const msgID = 45
	first := StartGeneration(msgID, "alice")
	second := StartGeneration(msgID, "alice")
	defer EndGeneration(msgID)

	if first.Err() == nil {
		t.Fatal("expected previous generation context to be cancelled on replace")
	}
	if second.Err() != nil {
		t.Fatal("expected new generation context to be active")
	}
}
