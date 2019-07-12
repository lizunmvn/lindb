package tree

import (
	"bytes"
	"fmt"
)

//Reader is used to parse B+Tree in disk, support get and prefix query
type Reader struct {
	buf             []byte
	position        int //current reader position
	length          int
	high            int         //Height of the tree
	levelPos        map[int]int //key:height of the tree  value:start position
	bodyPos         int         //The starting position of the data block
	hasChildrenNode bool        //Mark if there are child nodes
}

type InternalNode struct {
	hasParent    bool //
	suffixLen    int
	suffix       []byte
	nextStartPos int
}

type BlockHeader struct {
	count  int    //internal node count
	lcpLen int    //longest common prefix length
	lcp    []byte //longest common prefix
}

type LeafNodeInfo struct {
	suffixLen int
	suffix    []byte
	v         int
}

//Create a B+Tree Reader
func NewReader(treeBytes []byte) *Reader {
	reader := &Reader{
		buf:             treeBytes,
		length:          len(treeBytes),
		levelPos:        make(map[int]int),
		hasChildrenNode: false,
	}

	if HasChildrenNode == reader.readByte() {
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

func (r *Reader) Duplicator() *Reader {
	reader := &Reader{
		buf:             r.buf,
		levelPos:        r.levelPos,
		hasChildrenNode: r.hasChildrenNode,
		bodyPos:         r.bodyPos,
		high:            r.high,
	}
	return reader
}

// Get returns the value associated with k and true if it exists. Otherwise Get
// returns (zero-value, false).
func (r *Reader) Get(target []byte) (v int /*V*/, ok bool) {
	var startPos = 0

	if r.hasChildrenNode {
		for i := 1; i < r.high; i++ {
			r.newPosition(r.bodyPos + r.levelPos[i] + startPos)
			header := r.readNodeHeaderInfo()

			if header.lcpLen >= len(target) {
				if bytes.Compare(header.lcp, target) < 0 {
					return NotFound, false
				}
			}
			//
			startPos = r.binarySearchLeafNodePos(target, header)
			//startPos = r.findLeafNodePosByLcp(target, lcp, lcpLen, nodes)
		}
	}

	return r.binarySearch(startPos, target)
	//return r.linearSearch(startPos, target)
}

//Leaf node binary search
func (r *Reader) binarySearch(pos int, target []byte) (int /*V*/, bool) {
	r.newPosition(r.bodyPos + r.levelPos[r.high] + pos)
	header := r.readNodeHeaderInfo()

	if header.lcpLen > 0 {
		if !bytes.HasPrefix(target, header.lcp) {
			return NotFound, false
		}
	}

	offsetPos := r.readOffsetInfo(header.count)

	//binary search
	var mid, start, end int
	end = header.count - 1

	leafStartPos := r.position
	for ; start <= end; {
		mid = (end-start)/2 + start
		r.newPosition(offsetPos[mid] + leafStartPos)
		leafNode := r.readLeafNode()

		switch cmp := bytes.Compare(target[header.lcpLen:], leafNode.suffix); {
		case cmp > 0:
			start = mid + 1
		case cmp == 0:
			return leafNode.v, true
		default:
			end = mid - 1
		}
	}
	return NotFound, false
}

func getHeaderAndTargetLcp(target, headerLcp []byte) int {
	keyArray := make([][]byte, 2)
	keyArray[0] = target
	keyArray[1] = headerLcp

	return len(lcpByte(keyArray))
}

func (r *Reader) binarySearchLeafNodePos(target []byte, header *BlockHeader) (nextStartPos int) {
	//binary search
	var mid, start, end int
	offsetPos := r.readOffsetInfo(header.count)
	end = header.count - 1
	//
	targetLcpLen := getHeaderAndTargetLcp(header.lcp, target)
	lcpDiff := header.lcpLen - targetLcpLen
	targetDiff := target[targetLcpLen:]

	if lcpDiff > 0 {
		if bytesCompare(header.lcp[targetLcpLen:], targetDiff) < 0 {
			return NotFound
		}
		if bytesCompare(header.lcp[targetLcpLen:], targetDiff) > 0 {
			return r.readNodeInfo().nextStartPos
		}
	}

	startPos := r.position
	for start <= end {
		mid = (start + end) >> 1
		r.newPosition(offsetPos[mid] + startPos)
		internalNode := r.readNodeInfo()

		switch cmp := bytesCompare(targetDiff, internalNode.suffix); {
		case cmp > 0:
			start = mid + 1
		case cmp < 0:
			end = mid - 1
			nextStartPos = internalNode.nextStartPos
		default:
			if internalNode.hasParent {
				r.newPosition(offsetPos[mid+1] + startPos)
				return r.readNodeInfo().nextStartPos
			} else {
				return internalNode.nextStartPos
			}

		}
	}
	return nextStartPos
}

//
//func (r *Reader) binarySearchLeafNodePosByNoCommonPrefix(target []byte, nodes int) (nextStartPos int) {
//
//	offsetPos := r.readOffsetInfo(nodes)
//	var mid, start, end int
//	end = nodes - 1
//
//	//binary search
//	startPos := r.position
//	for start <= end {
//		mid = (start + end) >> 1
//		r.newPosition(offsetPos[mid] + startPos)
//		//read block info
//		block := r.readNodeInfo()
//
//		switch cmp := bytesCompare(target, block.suffix); {
//		case cmp > 0:
//			start = mid + 1
//		case cmp == 0:
//			if block.hasParent {
//				return r.readNodeInfo().nextStartPos
//			} else {
//				return block.nextStartPos
//			}
//		default:
//			end = mid - 1
//			nextStartPos = block.nextStartPos
//		}
//	}
//	return nextStartPos
//}

//Leaf node linear search
func (r *Reader) linearSearch(pos int, target []byte) (int /*V*/, bool) {
	r.newPosition(r.bodyPos + r.levelPos[r.high] + pos)
	header := r.readNodeHeaderInfo()

	if header.lcpLen > 0 {
		if !bytes.HasPrefix(target, header.lcp) {
			fmt.Println("xxxxxxxxx")
			return NotFound, false
		}
	}

	for i := 0; i < header.count; i++ {
		leafNode := r.readLeafNode()
		if bytes.Compare(target[header.lcpLen:], leafNode.suffix) == 0 {
			return int(leafNode.v), true
		}
	}
	return NotFound, false
}

//TODO zun.li
func (r *Reader) findLeafNodePosByLcp(target, lcp []byte, lcpLen, nodes int) (nextStartPos int) {
	//When the length of the longest common prefix is greater than the target,
	//and the byte code is smaller than the target, it directly returns -1.
	//like  if lcp("host-11") < target("host-2") return -1
	if lcpLen >= len(target) {
		if bytes.Compare(lcp, target) < 0 {
			return -1
		}
	}
	offsetLen := int(r.readUInt())
	//skip offset
	r.newPosition(r.position + offsetLen)

	for i := 0; i < nodes; i++ {
		hasParent := r.readByte()
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

func (r *Reader) findLeafNodePosByNoCommonPrefix(target []byte, nodes int) (nextStartPos int) {
	offsetLen := int(r.readUInt())
	r.newPosition(r.position + offsetLen)

	for i := 0; i < nodes; i++ {
		hasParent := r.readByte()
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
			key = bytesCombine(leafLcp, b)
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
			hasParent := r.readByte()
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
				key := bytesCombine(lcp, suffix)
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

//read next internal block info
func (r *Reader) readNodeInfo() *InternalNode {
	if r.position >= r.length {
		return nil
	}

	block := &InternalNode{}

	hasParent := r.readByte()
	if hasParent == HasParent {
		block.hasParent = true
	} else {
		block.hasParent = false
	}
	block.suffixLen = int(r.readUInt())
	if block.suffixLen > 0 {
		block.suffix = r.readBytes(block.suffixLen)
	}
	block.nextStartPos = int(r.readUInt())
	return block
}

func (r *Reader) readNodeHeaderInfo() *BlockHeader {
	block := &BlockHeader{}

	block.count = int(r.readUInt())
	length, lcp := r.readLcp()
	block.lcpLen = length
	if block.lcpLen > 0 {
		block.lcp = lcp
	}
	return block
}

func (r *Reader) readLcp() (length int, lcp []byte) {
	length = int(r.readUInt())
	if length > 0 {
		lcp = r.readBytes(length)
	}
	return length, lcp
}

func (r *Reader) readLeafNode() *LeafNodeInfo {
	node := &LeafNodeInfo{}
	node.suffixLen = int(r.readUInt())
	if node.suffixLen > 0 {
		node.suffix = r.readBytes(node.suffixLen)
	}
	node.v = int(r.readUInt())
	return node
}

func (r *Reader) readOffsetInfo(count int) []int {
	r.readUInt() //offset len
	offsetPos := make([]int, count)

	for i := 0; i < count; i++ {
		offsetPos[i] = int(r.readUInt())
	}
	return offsetPos
}

func (r *Reader) readUInt() uint64 {
	v, length := readReadUint(r.buf[r.position:])
	r.position += length
	return v
	//return uint64(r.readInt32())
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

//func (r *Reader) readInt32() uint32 {
//	x := binary.BigEndian.Uint32(r.readBytes(4))
//	return x
//}

func (r *Reader) readBytes(length int) []byte {
	b := r.buf[r.position : r.position+length]
	r.position += length
	return b
}

func (r *Reader) readByte() byte {
	b := r.buf[r.position]
	r.position += 1
	return b
}

func (r *Reader) newPosition(newPos int) {
	r.position = newPos
}
