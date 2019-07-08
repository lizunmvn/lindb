package tree

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

//Reader is used to parse B+Tree in disk, support get and prefix query
type Reader struct {
	buf             []byte
	position        int         //current reader position
	high            int         //Height of the tree
	levelPos        map[int]int //key:height of the tree  value:start position
	bodyPos         int         //The starting position of the data block
	hasChildrenNode bool        //Mark if there are child nodes
}

//Create a B+Tree Reader
func NewReader(treeBytes []byte) *Reader {
	reader := &Reader{
		buf:             treeBytes,
		levelPos:        make(map[int]int),
		hasChildrenNode: false,
	}

	if HasChildrenNode == int(reader.readUInt()) {
		reader.hasChildrenNode = true
	}

	reader.high = int(reader.readUInt())

	for i := 0; i < reader.high; i++ {
		level := int(reader.readUInt())

		start := int(reader.readUInt())
		reader.levelPos[level] = start
	}

	reader.bodyPos = reader.position
	return reader
}

// Get returns the value associated with k and true if it exists. Otherwise Get
// returns (zero-value, false).
func (r *Reader) Get(target []byte) (v int /*V*/, ok bool) {
	var startPos int
	startPos = 0

	if r.hasChildrenNode {
		for i := 1; i < r.high; i++ {
			startPos = r.findPos(i, startPos, target)
			if startPos < 0 {
				return NotFound, false
			}
		}
	}

	//return r.binarySearch(startPos, target)
	return r.linearSearch(startPos, target)
	//return -1, false
}

//Leaf node linear search
func (r *Reader) linearSearch(pos int, target []byte) (v int /*V*/, ok bool) {
	r.newPosition(r.bodyPos + r.levelPos[r.high] + pos)
	leafNodes := int(r.readUInt())
	leafLcpLen := int(r.readUInt())
	//
	if leafLcpLen > 0 {
		lcp := r.readBytes(leafLcpLen)
		if !bytes.HasPrefix(target, lcp) {
			fmt.Println("xxxxxxxxx")
			return NotFound, false
		}
	}
	//r.newPosition(r.position + leafLcpLen)

	//offsetLen := int(r.readUInt())
	//r.newPosition(r.position + offsetLen)
	for i := 0; i < leafNodes; i++ {
		suffixLen := int(r.readUInt())
		b := r.readBytes(suffixLen)
		v := r.readUInt()
		if bytes.Compare(target[leafLcpLen:], b) == 0 {
			return int(v), true
		}
	}
	return NotFound, false
}

//Leaf node binary search
func (r *Reader) binarySearch(pos int, target []byte) (v int /*V*/, ok bool) {
	r.newPosition(r.bodyPos + r.levelPos[r.high] + pos)
	leafNodes := int(r.readUInt())
	leafLcpLen := int(r.readUInt())
	//leaf-node lcp
	r.newPosition(r.position + leafLcpLen)
	//
	r.readUInt()
	offsetPos := make([]int, leafNodes)

	for i := 0; i < leafNodes; i++ {
		offsetPos[i] = int(r.readUInt())
	}

	var mid, start, end int
	end = len(offsetPos) - 1

	leafStartPos := r.position
	for ; start <= end; {
		mid = (end-start)/2 + start
		r.newPosition(offsetPos[mid] + leafStartPos)
		suffixLen := int(r.readUInt())
		b := r.readBytes(suffixLen)
		v := r.readUInt()
		if bytes.Compare(target[leafLcpLen:], b) < 0 {
			end = mid - 1
		} else if bytes.Compare(target[leafLcpLen:], b) > 0 {
			start = mid + 1
		} else {
			return int(v), true
		}
	}
	return NotFound, false
}

func (r *Reader) findPos(high, start int, target []byte) (nextStartPos int) {
	startPos := r.levelPos[high]
	r.newPosition(r.bodyPos + startPos + start)

	nodes := int(r.readUInt())
	//decode longest common prefix
	lcpLen := int(r.readUInt())
	var lcp []byte
	if lcpLen > 0 {
		lcp = r.readBytes(lcpLen)
		return r.findByLcp(target, lcp, lcpLen, nodes)
	} else {
		return r.findByNoCommonPrefix(target, nodes)
	}
}

func (r *Reader) findByLcp(target, lcp []byte, lcpLen, nodes int) (nextStartPos int) {
	//When the length of the longest common prefix is greater than the target,
	//and the byte code is smaller than the target, it directly returns -1.
	//like  if lcp("host-11") < target("host-2") return -1
	if lcpLen >= len(target) {
		if bytes.Compare(lcp, target) < 0 {
			return -1
		}
	}

	for i := 0; i < nodes; i++ {
		hasParent := r.readUInt()
		suffixLen := int(r.readUInt())
		if suffixLen == 0 {
			nextStartPos = int(r.readUInt())
			if hasParent == HasParent {
				if bytes.Compare(lcp, target) > 0 {
					return nextStartPos
				}
			} else if hasParent == NoParent {
				if bytes.Compare(lcp, target) >= 0 {
					return nextStartPos
				}
			}
		} else {
			suffix := r.readBytes(suffixLen)
			nextStartPos = int(r.readUInt())

			if lcpLen >= len(target) {
				if bytes.Compare(lcp, target) >= 0 {
					return nextStartPos
				}
			} else {
				if bytes.Compare(target, lcp) < 0 {
					return nextStartPos
				}
			}

			if hasParent == HasParent {
				if bytes.Compare(target[lcpLen:], suffix) < 0 {
					return nextStartPos
				}
			} else if hasParent == NoParent {
				if bytes.Compare(target[lcpLen:], suffix) <= 0 {
					return nextStartPos
				}
			}
		}
	}
	return NotFound
}

func (r *Reader) findByNoCommonPrefix(target []byte, nodes int) (nextStartPos int) {
	for i := 0; i < nodes; i++ {
		hasParent := r.readUInt()
		keyLen := int(r.readUInt())

		key := r.readBytes(keyLen)
		nextStartPos = int(r.readUInt())

		if hasParent == HasParent {
			if bytes.Compare(target, key) < 0 {
				return nextStartPos
			}
		} else if hasParent == NoParent {
			if bytes.Compare(target, key) <= 0 {
				return nextStartPos
			}
		}
	}
	return NotFound
}

func (r *Reader) seek(prefix []byte) {
	var startPos int
	startPos = 0
	for i := 1; i < r.high; i++ {
		startPos = r.seekPos(i, startPos, prefix)
		if startPos < 0 {
			return
		}
	}

	r.newPosition(r.bodyPos + r.levelPos[r.high] + startPos)
	r.readLeafNodes(prefix)
}

func (r *Reader) readLeafNodes(prefix []byte) {
	if r.position >= len(r.buf) {
		return
	}
	leafNodes := int(r.readUInt())
	leafLcpLen := int(r.readUInt())
	var leafLcp []byte
	if leafLcpLen > 0 {
		leafLcp = r.readBytes(leafLcpLen)
	}

	offsetLen := int(r.readUInt())
	r.newPosition(r.position + offsetLen)
	for i := 0; i < leafNodes; i++ {
		suffixLen := int(r.readUInt())
		b := r.readBytes(suffixLen)
		v := r.readUInt()
		var key []byte
		if nil != leafLcp {
			key = BytesCombine(leafLcp, b)
		} else {
			key = b
		}
		if bytes.HasPrefix(key, prefix) {
			fmt.Println("seek..", string(key), "xxx", v)
		}
	}
	r.readLeafNodes(prefix)
}

func (r *Reader) seekPos(high, start int, prefix []byte) (nextHighStart int) {
	startPos := r.levelPos[high]
	r.newPosition(r.bodyPos + startPos + start)

	nodes := int(r.readUInt())
	lcpLen := int(r.readUInt())
	var lcp []byte
	if lcpLen > 0 {
		lcp = r.readBytes(lcpLen)
	}
	if nil != lcp {
		for i := 0; i < nodes; i++ {
			hasParent := r.readUInt()
			suffixLen := int(r.readUInt())
			if suffixLen == 0 {
				nextHighStart = int(r.readUInt())
				if hasParent == HasParent {
					if bytes.Compare(prefix, lcp) < 0 {
						return nextHighStart
					}
				} else if hasParent == NoParent {
					if bytes.Compare(prefix, lcp) <= 0 {
						return nextHighStart
					}
				}

			} else {
				suffix := r.readBytes(suffixLen)
				key := BytesCombine(lcp, suffix)
				nextHighStart = int(r.readUInt())
				if hasParent == HasParent {
					if bytes.Compare(prefix, key) < 0 {
						return nextHighStart
					}
				} else if hasParent == NoParent {
					if bytes.Compare(prefix, key) <= 0 {
						return nextHighStart
					}
				}
			}
		}
	}
	return NotFound
}

func (r *Reader) readUInt() uint64 {
	v, length := readReadUvarint(r.buf[r.position:])
	r.position += length
	return v
	//return uint64(r.readInt32())
}

func readReadUvarint(buf []byte) (v uint64, len int) {
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

func (r *Reader) readInt32() uint32 {
	x := binary.BigEndian.Uint32(r.readBytes(4))
	return x
}

func (r *Reader) readBytes(length int) []byte {
	b := r.buf[r.position : r.position+length]
	r.position += length
	return b
}

func (r *Reader) newPosition(newPos int) {
	r.position = newPos
}

func BytesCombine(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, []byte(""))
}
