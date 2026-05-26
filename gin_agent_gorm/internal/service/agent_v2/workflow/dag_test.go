package workflow

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

func TestWorkflowOrderedNodesSortsDAGDependencies(t *testing.T) {
	flow := DAG(
		"test",
		"0.1.0",
		[]domain.AgentNode{
			testNode{key: "prompt_agent"},
			testNode{key: "intent_router"},
			testNode{key: "requirement_agent"},
			testNode{key: "memory_agent"},
		},
		map[string][]string{
			"requirement_agent": {"intent_router"},
			"memory_agent":      {"requirement_agent"},
			"prompt_agent":      {"memory_agent", "requirement_agent"},
		},
	)

	nodes, err := flow.OrderedNodes()
	if err != nil {
		t.Fatalf("OrderedNodes() error = %v", err)
	}

	got := make([]string, 0, len(nodes))
	for _, node := range nodes {
		got = append(got, node.Key())
	}
	want := []string{"intent_router", "requirement_agent", "memory_agent", "prompt_agent"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered node %d = %q, want %q; full order = %#v", i, got[i], want[i], got)
		}
	}
}

func TestWorkflowOrderedNodesRejectsMissingDependency(t *testing.T) {
	flow := DAG(
		"test",
		"0.1.0",
		[]domain.AgentNode{testNode{key: "prompt_agent"}},
		map[string][]string{
			"prompt_agent": {"memory_agent"},
		},
	)

	if _, err := flow.OrderedNodes(); err == nil {
		t.Fatal("OrderedNodes() error = nil, want missing dependency error")
	}
}

func TestWorkflowOrderedNodesRejectsCycle(t *testing.T) {
	flow := DAG(
		"test",
		"0.1.0",
		[]domain.AgentNode{
			testNode{key: "a"},
			testNode{key: "b"},
		},
		map[string][]string{
			"a": {"b"},
			"b": {"a"},
		},
	)

	if _, err := flow.OrderedNodes(); err == nil {
		t.Fatal("OrderedNodes() error = nil, want cycle error")
	}
}

func TestImageGenerationWorkflowIncludesVisionReviewAfterArtifacts(t *testing.T) {
	flow := ImageGenerationWorkflow(ImageGenerationWorkflowOptions{})

	nodes, err := flow.OrderedNodes()
	if err != nil {
		t.Fatalf("OrderedNodes() error = %v", err)
	}

	got := make([]string, 0, len(nodes))
	for _, node := range nodes {
		got = append(got, node.Key())
	}
	want := []string{
		"intent_router",
		"requirement_agent",
		"memory_agent",
		"prompt_agent",
		"image_generation_agent",
		"artifact_agent",
		"vision_review_agent",
	}
	if len(got) != len(want) {
		t.Fatalf("ordered nodes = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("ordered node %d = %q, want %q; full order = %#v", index, got[index], want[index], got)
		}
	}
}

type testNode struct {
	key string
}

func (node testNode) Key() string {
	return node.key
}

func (node testNode) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	return domain.StepResult{Status: domain.StepStatusCompleted}, nil
}
