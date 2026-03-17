package backup

import "testing"

func TestLogHubAppendSubscribeAndComplete(t *testing.T) {
	hub := NewLogHub()
	channel, cancel := hub.Subscribe(1, 4)
	defer cancel()
	first := hub.Append(1, "info", "started")
	if first.Sequence != 1 || first.Message != "started" {
		t.Fatalf("unexpected first event: %#v", first)
	}
	snapshot := hub.Snapshot(1)
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot size 1, got %d", len(snapshot))
	}
	event := <-channel
	if event.Message != "started" {
		t.Fatalf("unexpected streamed event: %#v", event)
	}
	hub.Complete(1, "success")
	completeEvent := <-channel
	if !completeEvent.Completed || completeEvent.Status != "success" {
		t.Fatalf("unexpected completion event: %#v", completeEvent)
	}
}
