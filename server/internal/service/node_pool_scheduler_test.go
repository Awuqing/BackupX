package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
)

// nodeRepoStub 返回预设节点切片；仅关注 List/FindByID。
// 其余方法返回零值，避免在调度路径被调用到。
type nodeRepoStub struct {
	nodes []model.Node
}

func (s *nodeRepoStub) List(context.Context) ([]model.Node, error) { return s.nodes, nil }
func (s *nodeRepoStub) FindByID(_ context.Context, id uint) (*model.Node, error) {
	for i := range s.nodes {
		if s.nodes[i].ID == id {
			return &s.nodes[i], nil
		}
	}
	return nil, nil
}
func (s *nodeRepoStub) FindByToken(context.Context, string) (*model.Node, error) { return nil, nil }
func (s *nodeRepoStub) FindLocal(context.Context) (*model.Node, error)           { return nil, nil }
func (s *nodeRepoStub) Create(context.Context, *model.Node) error                { return nil }
func (s *nodeRepoStub) BatchCreate(context.Context, []*model.Node) error         { return nil }
func (s *nodeRepoStub) Update(context.Context, *model.Node) error                { return nil }
func (s *nodeRepoStub) Delete(context.Context, uint) error                       { return nil }
func (s *nodeRepoStub) MarkStaleOffline(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestSelectPoolNode_PicksLeastLoaded(t *testing.T) {
	nodes := []model.Node{
		{ID: 1, Name: "node-a", Status: model.NodeStatusOnline, Labels: "prod,db"},
		{ID: 2, Name: "node-b", Status: model.NodeStatusOnline, Labels: "prod,db"},
		{ID: 3, Name: "node-offline", Status: model.NodeStatusOffline, Labels: "prod,db"},
		{ID: 4, Name: "node-other-pool", Status: model.NodeStatusOnline, Labels: "staging"},
	}
	svc := &BackupExecutionService{
		nodeRepo: &nodeRepoStub{nodes: nodes},
		records:  nil, // 触发 countRunningOnNode 返回 0，节点并列时按 ID 升序
	}
	chosen, err := svc.selectPoolNode(context.Background(), "db")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if chosen == nil || chosen.ID != 1 {
		t.Fatalf("expected node-a (ID=1), got %#v", chosen)
	}
}

func TestSelectPoolNode_EmptyPoolReturnsError(t *testing.T) {
	svc := &BackupExecutionService{
		nodeRepo: &nodeRepoStub{nodes: []model.Node{
			{ID: 1, Status: model.NodeStatusOnline, Labels: "prod"},
		}},
	}
	_, err := svc.selectPoolNode(context.Background(), "missing-pool")
	if err == nil {
		t.Fatal("expected empty-pool error")
	}
	var apperr *apperror.AppError
	if !errors.As(err, &apperr) || apperr.Code != "NODE_POOL_EMPTY" {
		t.Errorf("expected NODE_POOL_EMPTY, got %v", err)
	}
}

func TestSelectPoolNode_NilRepoDegradesGracefully(t *testing.T) {
	svc := &BackupExecutionService{}
	got, err := svc.selectPoolNode(context.Background(), "any")
	if err != nil {
		t.Errorf("nil repo should degrade silently, got err %v", err)
	}
	if got != nil {
		t.Errorf("nil repo should return nil node, got %v", got)
	}
}
