package taskgraph

import (
	"fmt"
	"slices"
)

type TaskNode struct {
	ID           string
	Dependencies []string
}

type TaskGraph struct {
	Nodes []TaskNode
}

func Validate(graph TaskGraph) error {
	nodes, err := indexNodes(graph)
	if err != nil {
		return err
	}

	for _, node := range graph.Nodes {
		seenDependencies := make(map[string]struct{}, len(node.Dependencies))
		for _, dependencyID := range node.Dependencies {
			if _, seen := seenDependencies[dependencyID]; seen {
				return fmt.Errorf("task %q declares duplicate dependency %q", node.ID, dependencyID)
			}
			seenDependencies[dependencyID] = struct{}{}
			if _, ok := nodes[dependencyID]; !ok {
				return fmt.Errorf("task %q depends on missing task %q", node.ID, dependencyID)
			}
		}
	}

	state := make(map[string]uint8, len(nodes))
	ids := sortedNodeIDs(nodes)
	for _, id := range ids {
		if state[id] != 0 {
			continue
		}
		if err := validateAcyclic(id, nodes, state); err != nil {
			return err
		}
	}

	return nil
}

func TopologicalLayers(graph TaskGraph) ([][]TaskNode, error) {
	nodes, err := indexNodes(graph)
	if err != nil {
		return nil, err
	}
	if err := Validate(graph); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for id := range nodes {
		inDegree[id] = 0
	}
	for _, node := range graph.Nodes {
		for _, dependencyID := range node.Dependencies {
			inDegree[node.ID]++
			dependents[dependencyID] = append(dependents[dependencyID], node.ID)
		}
	}

	ready := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	slices.Sort(ready)

	layers := make([][]TaskNode, 0)
	processed := 0

	for len(ready) > 0 {
		layerIDs := slices.Clone(ready)
		layer := make([]TaskNode, 0, len(layerIDs))
		nextReady := make([]string, 0)
		for _, id := range layerIDs {
			layer = append(layer, nodes[id])
			processed++
			for _, dependentID := range dependents[id] {
				inDegree[dependentID]--
				if inDegree[dependentID] == 0 {
					nextReady = append(nextReady, dependentID)
				}
			}
		}
		layers = append(layers, layer)
		slices.Sort(nextReady)
		ready = nextReady
	}

	if processed != len(nodes) {
		return nil, fmt.Errorf("task graph contains a dependency cycle")
	}

	return layers, nil
}

func indexNodes(graph TaskGraph) (map[string]TaskNode, error) {
	nodes := make(map[string]TaskNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("task id cannot be empty")
		}
		if _, exists := nodes[node.ID]; exists {
			return nil, fmt.Errorf("task %q is defined more than once", node.ID)
		}
		nodes[node.ID] = node
	}
	return nodes, nil
}

func validateAcyclic(id string, nodes map[string]TaskNode, state map[string]uint8) error {
	if state[id] == 1 {
		return fmt.Errorf("cycle detected at task %q", id)
	}
	if state[id] == 2 {
		return nil
	}

	state[id] = 1
	for _, dependencyID := range nodes[id].Dependencies {
		if err := validateAcyclic(dependencyID, nodes, state); err != nil {
			return err
		}
	}
	state[id] = 2
	return nil
}

func sortedNodeIDs(nodes map[string]TaskNode) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
