package model

import (
	"testing"
	"time"
)

func TestNodeEffectiveStatus(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		node *Node
		want string
	}{
		{
			name: "remote fresh heartbeat → stored online",
			node: &Node{IsLocal: false, Status: NodeStatusOnline, LastSeen: now.Add(-10 * time.Second)},
			want: NodeStatusOnline,
		},
		{
			name: "remote stale heartbeat but stored online → derived offline",
			node: &Node{IsLocal: false, Status: NodeStatusOnline, LastSeen: now.Add(-90 * time.Second)},
			want: NodeStatusOffline,
		},
		{
			name: "remote just past grace period → offline",
			node: &Node{IsLocal: false, Status: NodeStatusOnline, LastSeen: now.Add(-(OfflineGracePeriod + time.Second))},
			want: NodeStatusOffline,
		},
		{
			name: "remote within grace period → online",
			node: &Node{IsLocal: false, Status: NodeStatusOnline, LastSeen: now.Add(-(OfflineGracePeriod - time.Second))},
			want: NodeStatusOnline,
		},
		{
			name: "local node ignores LastSeen → stored online",
			node: &Node{IsLocal: true, Status: NodeStatusOnline, LastSeen: now.Add(-24 * time.Hour)},
			want: NodeStatusOnline,
		},
		{
			name: "remote stored offline stays offline",
			node: &Node{IsLocal: false, Status: NodeStatusOffline, LastSeen: now.Add(-5 * time.Second)},
			want: NodeStatusOffline,
		},
		{
			name: "nil node → offline",
			node: nil,
			want: NodeStatusOffline,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.node.EffectiveStatus(now); got != tc.want {
				t.Fatalf("EffectiveStatus = %q, want %q", got, tc.want)
			}
		})
	}
}
