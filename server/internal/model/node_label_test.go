package model

import "testing"

func TestNodeHasLabel(t *testing.T) {
	cases := []struct {
		labels string
		tag    string
		want   bool
	}{
		{"prod,db,high-mem", "prod", true},
		{"prod,db,high-mem", "db", true},
		{"prod,db,high-mem", "backup", false},
		{"  prod ,  db  ", "db", true}, // trim 空白
		{"", "prod", false},
		{"prod", "", false}, // 空 tag 不匹配
	}
	for _, c := range cases {
		n := &Node{Labels: c.labels}
		if got := n.HasLabel(c.tag); got != c.want {
			t.Errorf("labels=%q tag=%q want %v got %v", c.labels, c.tag, c.want, got)
		}
	}
}

func TestNodeLabelSet(t *testing.T) {
	n := &Node{Labels: "prod, db ,,high-mem,prod"}
	set := n.LabelSet()
	for _, want := range []string{"prod", "db", "high-mem"} {
		if _, ok := set[want]; !ok {
			t.Errorf("expected label %q in set", want)
		}
	}
	if len(set) != 3 {
		t.Errorf("duplicates not deduped, got %v", set)
	}
}

func TestNilNodeHasLabelSafe(t *testing.T) {
	var n *Node
	if n.HasLabel("anything") {
		t.Error("nil node should never match any label")
	}
	if s := n.LabelSet(); s != nil {
		t.Errorf("nil node LabelSet should be nil, got %v", s)
	}
}
