package trie

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/golang/snappy"
	"sort"
)

var scratch [binary.MaxVarintLen64]byte

type Writer struct {
	t           *Trie                 // Trie
	internalBuf map[int]*bytes.Buffer //Key is the floor of B+Tree
}

func NewWriter(trie *Trie) *Writer {
	return &Writer{
		t:           trie,
		internalBuf: make(map[int]*bytes.Buffer),
	}
}

func (w *Writer) encode() []byte {

	w.encodeNode(w.t.root, 1)

	writer := newByteBuff()
	writeInt(writer, len(w.internalBuf))

	for level, buf := range w.internalBuf {
		//write level
		writeInt(writer, level)

		data := buf.Bytes()
		encoded := snappy.Encode(nil, data)
		fmt.Println("snappy:", len(data)/len(encoded), " data:", len(data))
		if len(encoded) < int(float64(len(data))*0.8) {
			writeByte(writer, SnappyCompress)
			writeInt(writer, len(encoded))
			writeBytes(writer, encoded)
		} else {
			writeByte(writer, NoCompress)
			writeBytes(writer, data)
		}
	}
	return writer.Bytes()
}

func (w *Writer) encodeNode(nodes []*Node, level int) int {
	var newLevel = level + 1
	buf := w.getOrCreateByteBuf(level)

	var startPos = buf.Len()
	writeInt(buf, len(nodes))

	sort.SliceStable(nodes, func(i, j int) bool {
		return bytesCompare(nodes[i].k, nodes[j].k) < 0
	})
	writer := newByteBuff()
	for _, node := range nodes {
		writeInt(writer, len(node.k))
		writeBytes(writer, node.k)
		if node.hasValue {
			writeByte(writer, HasValue)
			writeInt(writer, node.v)
		} else {
			writeByte(writer, NoValue)
		}

		if len(node.children) > 0 {
			w.encodeNode(node.children, newLevel)
		}
	}
	data := writer.Bytes()
	writeBytes(buf, data)
	return startPos
}

//Get the current height of the byte buffer
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
}

func writeByte(buf *bytes.Buffer, c byte) {
	buf.WriteByte(c)
}

func writeBytes(buf *bytes.Buffer, b []byte) {
	buf.Write(b)
}
