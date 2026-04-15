package domain

import (
	"errors"
	"sort"
)

func topoSort(resources []Resource) ([]Resource, error) {
	byID := make(map[string]Resource, len(resources))
	inDegree := make(map[string]int, len(resources))
	adjacency := make(map[string][]string, len(resources))

	for _, resource := range resources {
		byID[resource.GetID()] = resource
		inDegree[resource.GetID()] = 0
	}

	for _, resource := range resources {
		for _, dependency := range resource.GetRequires() {
			adjacency[dependency] = append(adjacency[dependency], resource.GetID())
			inDegree[resource.GetID()]++
		}
	}

	queue := make([]string, 0)
	for _, resource := range resources {
		if inDegree[resource.GetID()] == 0 {
			queue = append(queue, resource.GetID())
		}
	}
	sort.Strings(queue)

	var ordered []Resource
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ordered = append(ordered, byID[current])
		for _, dependent := range adjacency[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				sort.Strings(queue)
			}
		}
	}

	if len(ordered) != len(resources) {
		return nil, errors.New("manifest resource graph contains a cycle")
	}
	return ordered, nil
}
