package workflow

import (
	"fmt"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

// Workflow 工作流定义，包含多个 Agent 节点
type Workflow struct {
	Name         string
	Version      string
	Nodes        []domain.AgentNode
	Dependencies map[string][]string
}

// Sequential 创建一个顺序执行的工作流
func Sequential(name string, version string, nodes ...domain.AgentNode) Workflow {
	return Workflow{
		Name:         name,
		Version:      version,
		Nodes:        nodes,
		Dependencies: map[string][]string{},
	}
}

// DAG 创建具有显式节点依赖关系的工作流。
func DAG(
	name string,
	version string,
	nodes []domain.AgentNode,
	dependencies map[string][]string,
) Workflow {
	if dependencies == nil {
		dependencies = map[string][]string{}
	}
	return Workflow{
		Name:         name,
		Version:      version,
		Nodes:        nodes,
		Dependencies: dependencies,
	}
}

// OrderedNodes 返回确定的拓扑执行顺序。
func (flow Workflow) OrderedNodes() ([]domain.AgentNode, error) {
	nodesByKey := make(map[string]domain.AgentNode, len(flow.Nodes))
	nodeOrder := make(map[string]int, len(flow.Nodes))
	for i, node := range flow.Nodes {
		key := node.Key()
		if key == "" {
			return nil, fmt.Errorf("workflow %s has node with empty key", flow.Name)
		}
		if _, exists := nodesByKey[key]; exists {
			return nil, fmt.Errorf("workflow %s has duplicate node %q", flow.Name, key)
		}
		nodesByKey[key] = node
		nodeOrder[key] = i
	}

	for key, dependencies := range flow.Dependencies {
		if _, ok := nodesByKey[key]; !ok {
			return nil, fmt.Errorf("workflow %s dependency target %q is not registered", flow.Name, key)
		}
		for _, dependency := range dependencies {
			if _, ok := nodesByKey[dependency]; !ok {
				return nil, fmt.Errorf("workflow %s node %q depends on missing node %q", flow.Name, key, dependency)
			}
		}
	}

	visited := make(map[string]bool, len(flow.Nodes))
	visiting := make(map[string]bool, len(flow.Nodes))
	ordered := make([]domain.AgentNode, 0, len(flow.Nodes))
	for _, node := range flow.Nodes {
		if err := flow.visitNode(node.Key(), nodesByKey, nodeOrder, visited, visiting, &ordered); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func (flow Workflow) visitNode(
	key string,
	nodesByKey map[string]domain.AgentNode,
	nodeOrder map[string]int,
	visited map[string]bool,
	visiting map[string]bool,
	ordered *[]domain.AgentNode,
) error {
	if visited[key] {
		return nil
	}
	if visiting[key] {
		return fmt.Errorf("workflow %s has dependency cycle at node %q", flow.Name, key)
	}

	visiting[key] = true
	dependencies := append([]string{}, flow.Dependencies[key]...)
	sortNodeKeys(dependencies, nodeOrder)
	for _, dependency := range dependencies {
		if err := flow.visitNode(dependency, nodesByKey, nodeOrder, visited, visiting, ordered); err != nil {
			return err
		}
	}
	visiting[key] = false
	visited[key] = true
	*ordered = append(*ordered, nodesByKey[key])
	return nil
}

func sortNodeKeys(keys []string, nodeOrder map[string]int) {
	for i := 1; i < len(keys); i++ {
		key := keys[i]
		j := i - 1
		for j >= 0 && nodeOrder[keys[j]] > nodeOrder[key] {
			keys[j+1] = keys[j]
			j--
		}
		keys[j+1] = key
	}
}
