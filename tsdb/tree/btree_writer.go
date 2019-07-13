package tree

import (
	"bytes"
	"encoding/binary"
	"sort"
)

var scratch [binary.MaxVarintLen64]byte

//Encoding the B+tree into the encoder is not safe for multiple goroutines.
type Writer struct {
	t       *Tree                 // B+Tree
	highBuf map[int]*bytes.Buffer //Key is the floor of B+Tree
}

//New a B+Tree encoder
func NewEncoder(tree *Tree) *Writer {
	return &Writer{
		t:       tree,
		highBuf: make(map[int]*bytes.Buffer),
	}
}

//Serialize a B+Tree into bytes
func (w *Writer) encode() []byte {
	q := w.t.r
	//no nodes
	if q == nil {
		return nil
	}

	writer := newByteBuff()

	switch qType := q.(type) {
	case *x:
		//has branch node
		writeByte(writer, HasChildrenNode)
		//Extract the longest common prefix for each node
		w.extractLcpByBranchNode(1, qType)
		w.serializeBranchNode(1, qType)
	case *d:
		//only leaf node
		writeByte(writer, NoChildrenNode)
		//Extract the longest common prefix for leaf node
		commonPrefix, _ := w.extractLcpByLeafNode(1, qType)
		w.serializeLeafNode(1, qType, commonPrefix)
	}

	var highs []int
	for k := range w.highBuf {
		highs = append(highs, k)
	}
	sort.Ints(highs)
	//write the height of the tree
	writeInt(writer, len(highs))

	body := newByteBuff()
	//write node information of each layer according to the height of the tree
	for _, high := range highs {
		writeInt(writer, high)
		//starting position of each layer
		writeInt(writer, body.Len())

		highWriter := w.highBuf[high]
		buf := highWriter.Bytes()
		writeBytes(body, buf)
	}

	writeBytes(writer, body.Bytes())
	return writer.Bytes()
}

// Write the header information of the branch node,
// including the number of nodes and the longest common prefix
func (w *Writer) writeBranchNodeHeader(buf *bytes.Buffer, nodes int, commonPrefix []byte) {
	//write nodes
	writeInt(buf, nodes)
	writeInt(buf, len(commonPrefix))
	//write lcp
	writeBytes(buf, commonPrefix)
}

//Write the longest common prefix first,
//and then write the node information including the starting position of the next layer.
func (w *Writer) serializeBranchNode(currentHigh int, parentNode *x) int {
	branchWriter := w.getOrCreateByteBuf(currentHigh)
	start := branchWriter.Len()

	var nextHigh = currentHigh + 1
	currentCommonPrefix := extractLcpByNodes(parentNode)
	//write lcp
	w.writeBranchNodeHeader(branchWriter, parentNode.c+1, currentCommonPrefix)

	bodyWriter := newByteBuff()
	//used in binary search
	//headerWriter := newByteBuff()

	for i := 0; i < parentNode.c+1; i++ {
		//writeInt(headerWriter, bodyWriter.Len())

		var startPos int
		branchNode := &parentNode.x[i]
		children := branchNode.ch

		switch cType := children.(type) {
		case *x:
			//no-leaf node
			startPos = w.serializeBranchNode(nextHigh, cType)
		case *d:
			//leaf node
			startPos = w.serializeLeafNode(nextHigh, cType, branchNode.lcp)
		}

		if nil != branchNode.k {
			writeByte(bodyWriter, HasParent)
			//write suffix
			suffix := branchNode.k.([]byte)[len(currentCommonPrefix):]
			w.writeKey(bodyWriter, suffix)
		} else {
			writeByte(bodyWriter, NoParent)
			//write suffix
			noParentKey := branchNode.lastKey
			noParentSuffix := noParentKey[len(currentCommonPrefix):]
			w.writeKey(bodyWriter, noParentSuffix)
		}
		writeInt(bodyWriter, startPos)
	}
	//writeInt(branchWriter, headerWriter.Len())
	//writeBytes(branchWriter, headerWriter.Bytes())
	writeBytes(branchWriter, bodyWriter.Bytes())
	return start
}

//Write the longest common prefix of the leaf node first, then write the value information.
func (w *Writer) serializeLeafNode(currentHigh int, dType *d, leafCommonPrefix []byte) (start int) {
	leafWriter := w.getOrCreateByteBuf(currentHigh)

	//startPosition
	start = leafWriter.Len()
	//offsetWriter := newByteBuff()
	dataWriter := newByteBuff()

	var leafNodes int
	for i := 0; i < dType.c+1; i++ {
		if nil != dType.d[i].k {
			pair := dType.d[i]

			//writeInt(offsetWriter, dataWriter.Len())
			//write suffix
			w.writeKey(dataWriter, pair.k.([]byte))
			//write value
			writeInt(dataWriter, pair.v.(int))
			leafNodes++
		}
	}
	//write leaf node count
	writeInt(leafWriter, leafNodes)
	//write leaf node lcp
	w.writeKey(leafWriter, leafCommonPrefix)
	//write offset
	//writeInt(leafWriter, offsetWriter.Len())
	//writeBytes(leafWriter, offsetWriter.Bytes())
	//write k-v
	//encoded := snappy.Encode(nil, dataWriter.Bytes())
	writeBytes(leafWriter, dataWriter.Bytes())
	return start
}

//=============================================Extract the longest common prefix of each node
//Extract the longest common prefix inside the node
func extractLcpByNodes(parentNode *x) (longestCommonPrefix []byte) {
	var keyArray [][]byte
	for i := 0; i <= parentNode.c; i++ {
		branchNode := parentNode.x[i]
		if nil != branchNode.k {
			keyArray = append(keyArray, branchNode.k.([]byte))
		} else {
			keyArray = append(keyArray, branchNode.lastKey)
		}
	}
	longestCommonPrefix = lcpByte(keyArray)
	return longestCommonPrefix
}

//Extract the longest common prefix of the branch node.
//High indicates the height of the current tree, starting from 1.
func (w *Writer) extractLcpByBranchNode(high int, parentNode *x) (commonPrefix, lastKey []byte) {

	var branchCommonPrefix []byte //The longest common prefix with the same parent node
	var nextHigh = high + 1       //The next layer of tree height

	var keyArray [][]byte
	for i := 0; i <= parentNode.c; i++ {
		var hasParent = false // Whether the current node has a parent node
		branchNode := &parentNode.x[i]
		if nil != branchNode.k {
			hasParent = true
			keyArray = append(keyArray, branchNode.k.([]byte))
		} else {
			hasParent = false
		}

		children := branchNode.ch
		var commonPrefix, endKey []byte
		switch cType := children.(type) {
		case *x:
			//branch node
			commonPrefix, endKey = w.extractLcpByBranchNode(nextHigh, cType)
		case *d:
			//leaf node
			commonPrefix, endKey = w.extractLcpByLeafNode(nextHigh, cType)
		}
		//no-parent
		if !hasParent {
			branchNode.lastKey = endKey
			lastKey = endKey
			keyArray = append(keyArray, lastKey)
		}
		branchNode.lcp = commonPrefix
	}
	branchCommonPrefix = lcpByte(keyArray)
	return branchCommonPrefix, lastKey
}

//Extract the longest common prefix of the leaf node
func (w *Writer) extractLcpByLeafNode(i int, leafNode *d) (commonPrefix, lastKey []byte) {
	var keyArray [][]byte
	for i := 0; i <= leafNode.c; i++ {
		key := leafNode.d[i].k
		if nil != key {
			keyArray = append(keyArray, key.([]byte))
		}
	}
	//longest common prefix of leaf nodes
	commonPrefix = lcpByte(keyArray)

	//Remove the longest common prefix
	for i := 0; i <= leafNode.c; i++ {
		pair := leafNode.d[i]
		if nil != pair.k {
			suffix := (pair.k.([]byte))[len(commonPrefix):]
			lastKey = pair.k.([]byte)
			leafNode.d[i].k = suffix
		}
	}
	return commonPrefix, lastKey
}

//Get the current height of the byte buffer
func (w *Writer) getOrCreateByteBuf(high int) *bytes.Buffer {
	buf := w.highBuf[high]
	if nil == buf {
		buf = newByteBuff()
		w.highBuf[high] = buf
	}
	return buf
}

//Write node key to buf
func (w *Writer) writeKey(buf *bytes.Buffer, suffix []byte) {
	writeInt(buf, len(suffix))
	writeBytes(buf, suffix)
}

//Create a ByteBuff
func newByteBuff() *bytes.Buffer {
	var v []byte
	return bytes.NewBuffer(v)
}

//Write a variable int
func writeInt(buf *bytes.Buffer, v int) {
	n := binary.PutUvarint(scratch[:], uint64(v))
	writeBytes(buf, scratch[:n])
}

//Write a byte
func writeByte(buf *bytes.Buffer, c byte) {
	buf.WriteByte(c)
}

func writeBytes(buf *bytes.Buffer, b []byte) {
	if len(b) > 0 {
		buf.Write(b)
	}
}
