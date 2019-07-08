package tree

import (
	"bytes"
	"encoding/binary"
	"sort"
)

var scratch [binary.MaxVarintLen64]byte

//Encoding the B+tree into the encoder is not safe for multiple goroutines.
type Writer struct {
	t           *Tree                 // B+Tree
	internalBuf map[int]*bytes.Buffer //Key is the floor of B+Tree
}

//New a B+Tree encoder
func NewEncoder(tree *Tree) *Writer {
	return &Writer{
		t:           tree,
		internalBuf: make(map[int]*bytes.Buffer),
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
		//has internal node
		writeInt(writer, HasChildrenNode)
		//Extract the longest common prefix for each node
		w.extractLcpByInternalNode(1, qType)
		w.serializeInternalNode(1, qType)
	case *d:
		//only leaf node
		writeInt(writer, NoChildrenNode)
		//Extract the longest common prefix for leaf node
		commonPrefix, _ := w.extractLcpByLeafNode(1, qType)
		w.serializeLeafNode(1, qType, commonPrefix)
	}

	body := newByteBuff()

	//write internal nodes
	var highs []int
	for k := range w.internalBuf {
		highs = append(highs, k)
	}

	sort.Ints(highs)
	writeInt(writer, len(highs))

	// To perform the opertion you want
	for _, high := range highs {
		//write level
		writeInt(writer, high)

		internalWriter := w.internalBuf[high]
		buf := internalWriter.Bytes()

		writeInt(writer, body.Len())
		//write bytes length
		writeBytes(body, buf)
	}

	writeBytes(writer, body.Bytes())
	return writer.Bytes()
}

//Write the longest common prefix first,
//and then write the node information including the starting position of the next layer.
func (w *Writer) serializeInternalNode(currentHigh int, parentNode *x) int {
	internal := w.getOrCreateByteBuf(currentHigh)
	start := internal.Len()

	var nextHigh = currentHigh + 1
	var hasParent = false
	currentCommonPrefix := extractLcpByNodes(parentNode)

	//write nodes
	writeInt(internal, parentNode.c+1)
	//write lcp
	writeInt(internal, len(currentCommonPrefix))
	if len(currentCommonPrefix) > 0 {
		writeBytes(internal, currentCommonPrefix)
	}

	for i := 0; i < parentNode.c+1; i++ {
		childNode := &parentNode.x[i]
		var startPos int

		var internalKey []byte
		if nil != childNode.k {
			//used in lookup
			writeInt(internal, HasParent)
			hasParent = true
			internalKey = childNode.k.([]byte)
		} else {
			writeInt(internal, NoParent)
			hasParent = false
		}

		cc := childNode.ch

		switch cType := cc.(type) {
		case *x:
			//internal node
			startPos = w.serializeInternalNode(nextHigh, cType)
		case *d:
			//leaf node
			startPos = w.serializeLeafNode(nextHigh, cType, childNode.lcp)
		}
		//write suffix
		if hasParent {
			suffix := internalKey[len(currentCommonPrefix):]
			suffixLen := len(suffix)
			writeInt(internal, suffixLen)
			if suffixLen > 0 {
				writeBytes(internal, suffix)
			}
		} else {
			noParentLastKey := childNode.lastKey
			noParentSuffix := noParentLastKey[len(currentCommonPrefix):]
			writeInt(internal, len(noParentSuffix))
			if len(noParentSuffix) > 0 {
				writeBytes(internal, noParentSuffix)
			}
		}
		writeInt(internal, startPos)
	}
	return start
}

func extractLcpByNodes(parentNode *x) (currentCommonPrefix []byte) {
	var keyArray [][]byte
	for i := 0; i <= parentNode.c; i++ {
		internalNode := parentNode.x[i]
		if nil != internalNode.k {
			keyArray = append(keyArray, internalNode.k.([]byte))
		} else {
			keyArray = append(keyArray, internalNode.lastKey)
		}
	}
	currentCommonPrefix = lcpByte(keyArray)
	//fmt.Println("currentHigh:[", currentHigh, "]--nodes:[", parentNode.c+1, "]---commonPrefix--[", string(currentCommonPrefix), "]")
	return currentCommonPrefix
}

//Write the longest common prefix of the leaf node first, then write the value information.
func (w *Writer) serializeLeafNode(currentHigh int, dType *d, leafCommonPrefix []byte) (start int) {
	leafWriter := w.getOrCreateByteBuf(currentHigh)

	//startPosition
	start = leafWriter.Len()
	offsetWriter := newByteBuff()
	dataWriter := newByteBuff()

	var leafNodes int

	for i := 0; i < dType.c+1; i++ {
		if nil != dType.d[i].k {
			pair := dType.d[i]

			writeInt(offsetWriter, dataWriter.Len())
			//write suffix key
			writeInt(dataWriter, len(pair.k.([]byte)))
			if len(pair.k.([]byte)) > 0 {
				writeBytes(dataWriter, pair.k.([]byte))
			}
			//write value
			writeInt(dataWriter, pair.v.(int))
			leafNodes++
		}
	}
	//write leaf node count
	writeInt(leafWriter, leafNodes)
	//write leaf node lcp
	writeInt(leafWriter, len(leafCommonPrefix))
	if len(leafCommonPrefix) > 0 {
		writeBytes(leafWriter, leafCommonPrefix)
	}
	//write offset
	//writeInt(leafWriter, offsetWriter.Len())
	//writeBytes(leafWriter, offsetWriter.Bytes())
	//write k-v
	writeBytes(leafWriter, dataWriter.Bytes())
	return start
}

//Extract the longest common prefix of the no-leaf node.
//High indicates the height of the current tree, starting from 1.
func (w *Writer) extractLcpByInternalNode(high int, parentNode *x) (commonPrefix, lastKey []byte) {

	var internalCommonPrefix []byte //The longest common prefix with the same parent node
	var nextHigh = high + 1         //The next layer of tree height

	var keyArray [][]byte //
	for i := 0; i <= parentNode.c; i++ {
		var hasParent = false // Whether the current node has a parent node
		childNode := &parentNode.x[i]
		key := childNode.k
		if nil != key {
			hasParent = true
			keyArray = append(keyArray, key.([]byte))
		} else {
			hasParent = false
		}

		children := childNode.ch
		var childCommonPrefix []byte
		switch cType := children.(type) {
		case *x:
			// internal node
			commonPrefix, internalLastKey := w.extractLcpByInternalNode(nextHigh, cType)
			childCommonPrefix = commonPrefix
			//
			if !hasParent {
				childNode.lastKey = internalLastKey
				lastKey = internalLastKey
				keyArray = append(keyArray, lastKey)
			}

		case *d:
			//leaf node
			commonPrefix, leafLastKey := w.extractLcpByLeafNode(nextHigh, cType)
			childCommonPrefix = commonPrefix

			if !hasParent {
				childNode.lastKey = leafLastKey
				lastKey = leafLastKey
				keyArray = append(keyArray, leafLastKey)
			}
		}
		childNode.lcp = childCommonPrefix
	}
	internalCommonPrefix = lcpByte(keyArray)
	return internalCommonPrefix, lastKey
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
	//longest common prefix
	commonPrefix = lcpByte(keyArray)

	for i := 0; i <= leafNode.c; i++ {
		pair := leafNode.d[i]
		if nil != pair.k {
			suffix := (pair.k.([]byte))[len(commonPrefix):]
			lastKey = pair.k.([]byte)
			//Remove the longest common prefix
			leafNode.d[i].k = suffix
		}
	}
	return commonPrefix, lastKey
}


func (w *Writer) getOrCreateByteBuf(high int) *bytes.Buffer {
	buf := w.internalBuf[high]
	if nil == buf {
		buf = newByteBuff()
		w.internalBuf[high] = buf
	}
	return buf
}

func newByteBuff() *bytes.Buffer {
	var v []byte
	return bytes.NewBuffer(v)
}

func writeInt(buf *bytes.Buffer, v int) {
	n := binary.PutUvarint(scratch[:], uint64(v))
	writeBytes(buf, scratch[:n])
	//bs := make([]byte, 4)
	//binary.BigEndian.PutUint32(bs, uint32(v))
	//writeBytes(buf, bs)
}

func writeBytes(buf *bytes.Buffer, b []byte) {
	buf.Write(b)
}
