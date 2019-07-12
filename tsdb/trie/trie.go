package trie

import "fmt"

type Node struct {
	k        []byte
	v        int
	hasValue bool
	children []*Node
}

type Trie struct {
	root    []*Node
	writing bool
}

func newNode(key byte) *Node {
	return &Node{
		k:        []byte{key},
		children: make([]*Node, 0),
	}
}

func NewTrie() *Trie {

	return &Trie{
		root:    make([]*Node, 0),
		writing: true,
	}
}

func (t *Trie) put(key []byte, v int) {
	nodes := &t.root
	length := len(key)

	for i := 0; i < length; i++ {
		node := find(nodes, key[i])
		if nil == node {
			node = newNode(key[i])
			*nodes = append(*nodes, node)
		}

		if i == length-1 {
			node.v = v
			node.hasValue = true
		} else {
			nodes = &node.children
		}
	}
}

func find(nodes *[]*Node, key byte) *Node {
	if nil == nodes || len(*nodes) == 0 {
		return nil
	}
	for _, node := range *nodes {
		if nil != node && node.k[0] == key {
			return node
		}
	}
	return nil
}

func (t *Trie) get(key []byte) (int, bool) {
	return NotFound, true
}

func (t *Trie) extractCommonPrefix() {
	for {
		flag := extractPrefix(t.root)
		if !flag {
			break
		}

	}
	fmt.Println("xx")
}

func extractPrefix(nodes []*Node) bool {
	flag := false

	for index, node := range nodes {
		if len(node.children) == 1 && !node.hasValue {

			keys := bytesCombine(node.k, node.children[0].k)

			nodes[index] = &Node{
				k:        keys,
				v:        node.children[0].v,
				children: node.children[0].children,
			}
			flag = true
		}
		f2 := extractPrefix(nodes[index].children)
		if f2 {
			flag = true
		}
	}
	return flag
}
