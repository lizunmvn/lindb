package trie

import (
	"fmt"
	"github.com/golang/snappy"
)

type InternalNode struct {
	k           []byte
	hasValue    bool
	v           int
	childrenPos int
}

type Reader struct {
	br       *BytesReader
	high     int
	length   int
	bodyPos  int
	levelPos map[int]int
}

type BytesReader struct {
	buf      []byte
	position int
}

//Create a B+Tree Reader
func NewReader(treeBytes []byte) *Reader {
	reader := &Reader{
		br: &BytesReader{
			buf: treeBytes,
		},
		length:   len(treeBytes),
		levelPos: make(map[int]int),
	}

	reader.length = len(treeBytes)
	reader.high = int(reader.br.readUInt())

	for i := 0; i < reader.high; i++ {
		level := int(reader.br.readUInt())

		start := int(reader.br.readUInt())
		reader.levelPos[level] = start
	}

	reader.bodyPos = reader.br.position
	return reader
}

func (r *Reader) Get(k []byte) (int, bool) {
	var high = 1
	var startPos = 0
	for {
		find := false
		r.br.newPosition(r.bodyPos + r.levelPos[high] + startPos)
		nodes := int(r.br.readUInt())
		var reader = r.br
		if SnappyCompress == r.br.readByte() {
			fmt.Println("xxx")
			data, err := r.readSnappyData()
			if nil == err {
				reader = &BytesReader{
					buf: data,
				}
			} else {
				return NotFound, false
			}
		}
		for i := 0; i < nodes; i++ {
			node := reader.readInternalNode()
			kLen := len(node.k)
			if bytesCompare(node.k, k[:kLen]) == 0 {
				startPos = node.childrenPos
				k = k[kLen:]
				high += 1
				find = true
				if len(k) == 0 {
					return node.v, true
				}
				break
			}
		}
		if !find {
			return NotFound, false
		}
	}

}

func (r *BytesReader) readInternalNode() *InternalNode {
	node := &InternalNode{}
	kLen := int(r.readUInt())
	node.k = r.readBytes(kLen)

	hasValue := r.readByte() == HasValue
	node.hasValue = hasValue
	if hasValue {
		node.v = int(r.readUInt())
	}
	node.childrenPos = int(r.readUInt())
	return node

}

func (r *BytesReader) readUInt() uint64 {
	v, length := readReadUint(r.buf[r.position:])
	r.position += length
	return v
}

func readReadUint(buf []byte) (v uint64, len int) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		b := buf[i]
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return x, i + 1
			}
			return x | uint64(b)<<s, i + 1
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}

func (r *BytesReader) readBytes(length int) []byte {
	b := r.buf[r.position : r.position+length]
	r.position += length
	return b
}

func (r *BytesReader) readByte() byte {
	b := r.buf[r.position]
	r.position += 1
	return b
}

func (r *BytesReader) newPosition(newPos int) {
	r.position = newPos
}

func (r *Reader) readSnappyData() ([]byte, error) {
	length := int(r.br.readUInt())
	data := r.br.readBytes(length)
	return snappy.Decode(nil, data)
}
