package models

// StorageState represents storage cluster node state.
// NOTICE: it is not safe for concurrent use.
type StorageState struct {
	Name        string                 `json:"name"`
	ActiveNodes map[string]*ActiveNode `json:"activeNodes"`
}

// NewStorageState creates storage cluster state
func NewStorageState() *StorageState {
	return &StorageState{
		ActiveNodes: make(map[string]*ActiveNode),
	}
}

// AddActiveNode adds a node into active node list
func (s *StorageState) AddActiveNode(node *ActiveNode) {
	key := node.Node.Indicator()
	_, ok := s.ActiveNodes[key]
	if !ok {
		s.ActiveNodes[key] = node
	}
}

// RemoveActiveNode removes a node from active node list
func (s *StorageState) RemoveActiveNode(node string) {
	delete(s.ActiveNodes, node)
}

// GetActiveNodes returns all active nodes
func (s *StorageState) GetActiveNodes() []*ActiveNode {
	var nodes []*ActiveNode
	for _, node := range s.ActiveNodes {
		nodes = append(nodes, node)
	}
	return nodes
}
