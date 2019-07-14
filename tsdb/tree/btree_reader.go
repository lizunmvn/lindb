package tree

import (
	"bytes"
	"github.com/golang/snappy"
)

//Reader is used to parse B+Tree in disk, support get and prefix query
type Reader struct {
	bufReader       *ByteBufReader
	height          int         //height of the B-tree
	highPos         map[int]int //key:height of the tree  value:start position
	bodyPos         int         //The starting position of the data block
	hasChildrenNode bool
}

//ReaderIterator capture the state of enumerating a tree. It is returned from the Seek* methods.
type ReaderIterator struct {
	reader *Reader //tree reader

	//Hit data block information
	leafNodes int    //Number of nodes
	idx       int    //index
	lcp       []byte //longest common prefix

	key   []byte // k
	value int    // v

	hit    bool
	filter Filter
	init   bool // The tag is read for the first time
}

//Returns true if the iteration has more elements
func (it *ReaderIterator) Next() bool {
	if !it.init {
		it.init = true
		return true
	}
	if it.reader.bufReader.isEnd() {
		it.key = nil
		return false
	}
	if !it.hit {
		it.leafNodes = int(it.reader.bufReader.readUInt())
		_, lcp := it.reader.bufReader.readKey()
		it.lcp = lcp
		it.hit = true
	}
	if it.idx < it.leafNodes {
		_, key := it.reader.bufReader.readKey()
		v := int(it.reader.bufReader.readUInt())

		it.idx++
		if len(it.lcp) > 0 {
			key = bytesCombine(it.lcp, key)
		}
		if it.filter.endMatch(key) {
			it.key = key
			it.value = v
			return true
		} else {
			it.key = nil
			return false
		}
	} else {
		it.idx = 0
		it.hit = false
		return it.Next()
	}
}

//Returns the next key in the iteration.
func (it *ReaderIterator) GetKey() []byte {
	return it.key
}

//Returns the next value in the iteration.
func (it *ReaderIterator) GetValue() int {
	return it.value
}

//type InternalNode struct {
//	hasParent    bool //
//	suffixLen    int
//	suffix       []byte
//	nextStartPos int
//}

//type BlockHeader struct {
//	count  int    //internal node count
//	lcpLen int    //longest common prefix length
//	lcp    []byte //longest common prefix
//}

//type LeafNodeInfo struct {
//	suffixLen int
//	suffix    []byte
//	v         int
//}

//Create a B+Tree Reader
func NewReader(treeBytes []byte) *Reader {
	reader := &Reader{
		bufReader:       NewBufReader(treeBytes),
		highPos:         make(map[int]int),
		hasChildrenNode: false,
	}

	//reader header
	if HasChildrenNode == reader.bufReader.readByte() {
		reader.hasChildrenNode = true
	}
	reader.height = int(reader.bufReader.readUInt())
	//Starting offset of each height
	for i := 0; i < reader.height; i++ {
		high := int(reader.bufReader.readUInt())
		start := int(reader.bufReader.readUInt())
		reader.highPos[high] = start
	}

	reader.bodyPos = reader.bufReader.position
	return reader
}

//Creates a new reader that shares this buffer's content.
func (r *Reader) Duplicator() *Reader {
	reader := &Reader{
		bufReader:       r.bufReader,
		highPos:         r.highPos,
		hasChildrenNode: r.hasChildrenNode,
		bodyPos:         r.bodyPos,
		height:          r.height,
	}
	return reader
}

// Get returns the value associated with k and true if it exists. Otherwise Get
// returns (zero-value, false).
func (r *Reader) Get(target []byte) (v int /*V*/, ok bool) {
	startPos := r.findTargetLeafNodePos(target)
	if startPos == NotFound {
		return NotFound, false
	}
	//return r.binarySearch(startPos, target)
	return r.linearSearchTarget(startPos, target)
}

//Find the target leaf node position
func (r *Reader) findTargetLeafNodePos(target []byte) int {
	var startPos = 0

	if r.hasChildrenNode {
		for high := 1; high < r.height; high++ {
			r.bufReader.newPosition(r.bodyPos + r.highPos[high] + startPos)
			//read branch count
			count := int(r.bufReader.readUInt())
			//read common prefix
			lcpLen, lcp := r.bufReader.readKey()

			if lcpLen >= len(target) {
				if bytes.Compare(lcp, target) < 0 {
					return NotFound
				}
			}
			//startPos = r.binarySearchLeafNodePos(target, header)
			startPos = r.linearSearchTargetPos(target, count, lcpLen, lcp)
		}
	} else {
		return 0
	}
	return startPos
}

func extractHeaderAndTargetLcp(target, headerLcp []byte) int {
	var keyArray = make([][]byte, 2)
	keyArray[0] = target
	keyArray[1] = headerLcp

	return len(lcpByte(keyArray))
}

//Returns the position of the leftmost element in the target,
//return -1 if not found.
func (r *Reader) linearSearchTargetPos(target []byte, count, lcpLen int, lcp []byte) int {
	if lcpLen >= len(target) {
		if bytes.Compare(lcp, target) < 0 {
			return NotFound
		}
	}

	targetLcpLen := extractHeaderAndTargetLcp(lcp, target)
	lcpDiff := lcpLen - targetLcpLen
	targetSuffix := target[targetLcpLen:]

	if lcpDiff > 0 {
		cmp := bytesCompare(lcp[targetLcpLen:], targetSuffix)
		if cmp < 0 {
			return NotFound
		}
		if cmp > 0 {
			//read next branch node
			r.bufReader.readByte()
			r.bufReader.newPosition(r.bufReader.position + int(r.bufReader.readUInt()))
			return int(r.bufReader.readUInt())
		}
	}

	for i := 0; i < count; i++ {
		//read a branch node
		hasParent := r.bufReader.readByte() == HasParent
		suffixLen := int(r.bufReader.readUInt())
		suffix := r.bufReader.readBytes(suffixLen)
		nextStartPos := int(r.bufReader.readUInt())

		cmp := bytesCompare(suffix, targetSuffix)
		if cmp > 0 {
			return nextStartPos
		} else if cmp == 0 {
			if !hasParent {
				return nextStartPos
			}
		}
	}
	return NotFound
}

//Linear search target,
//return associated value; return false if no such key
func (r *Reader) linearSearchTarget(pos int, target []byte) (int /*V*/, bool) {
	r.bufReader.newPosition(r.bodyPos + r.highPos[r.height] + pos)

	//data, err := r.readSnappyData()
	//if nil == err {
	//	reader := NewBufReader(data)
	//	count := int(reader.readUInt())
	//	lcpLen, lcp := reader.readKey()
	//	if lcpLen > 0 {
	//		if !bytes.HasPrefix(target, lcp) {
	//			return NotFound, false
	//		}
	//	}
	//
	//	offsetPos := reader.readOffsetInfo(count)
	//
	//	//binary search
	//	var mid, start, end int
	//	end = count - 1
	//
	//	leafStartPos := reader.position
	//	for ; start <= end; {
	//		mid = (end-start)/2 + start
	//		reader.newPosition(offsetPos[mid] + leafStartPos)
	//		_, suffix := reader.readKey()
	//		v := int(reader.readUInt())
	//
	//		switch cmp := bytes.Compare(target[lcpLen:], suffix); {
	//		case cmp > 0:
	//			start = mid + 1
	//		case cmp == 0:
	//			return v, true
	//		default:
	//			end = mid - 1
	//		}
	//	}
	//
	//} else {
	//	return NotFound, false
	//}

	count := int(r.bufReader.readUInt())
	lcpLen, lcp := r.bufReader.readKey()
	//useSnappy := r.bufReader.readByte() == SnappyCompress
	useSnappy := false

	if lcpLen > 0 {
		if !bytes.HasPrefix(target, lcp) {
			return NotFound, false
		}
	}

	if useSnappy {
		data, err := r.readSnappyData()
		if nil == err {
			reader := NewBufReader(data)
			for i := 0; i < count; i++ {
				//read a leaf node
				suffix := reader.readBytes(int(reader.readUInt()))
				v := int(reader.readUInt())
				if bytes.Compare(target[lcpLen:], suffix) == 0 {
					return int(v), true
				}
			}
		}
	} else {

		for i := 0; i < count; i++ {
			//read a leaf node
			suffix := r.bufReader.readBytes(int(r.bufReader.readUInt()))
			v := int(r.bufReader.readUInt())
			if bytes.Compare(target[lcpLen:], suffix) == 0 {
				return int(v), true
			}
		}
	}
	return NotFound, false
}

func (r *Reader) readSnappyData() ([]byte, error) {
	length := int(r.bufReader.readUInt())
	data := r.bufReader.readBytes(length)
	return snappy.Decode(nil, data)
}

//
//func (r *Reader) findLeafNodePosByNoCommonPrefix(target []byte, nodes int) (nextStartPos int) {
//	offsetLen := int(r.readUInt())
//	r.newPosition(r.position + offsetLen)
//
//	for i := 0; i < nodes; i++ {
//		hasParent := r.readByte()
//		keyLen := int(r.readUInt())
//
//		key := r.readBytes(keyLen)
//		nextStartPos = int(r.readUInt())
//
//		if hasParent == HasParent {
//			if bytes.Compare(target, key) < 0 {
//				return nextStartPos
//			}
//		} else if hasParent == NoParent {
//			if bytes.Compare(target, key) <= 0 {
//				return nextStartPos
//			}
//		}
//	}
//	return NotFound
//}

func (r *Reader) Range(startKey, endKey []byte) *ReaderIterator {
	startPos := r.findTargetLeafNodePos(startKey)
	if startPos == NotFound {
		return nil
	}
	r.bufReader.newPosition(r.bodyPos + r.highPos[r.height] + startPos)

	rangeFilter := &RangeFilter{
		startKey: startKey,
		endKey:   endKey,
	}
	return r.seekLeafNodes(rangeFilter)
}

//SeekFirst returns an Iterator positioned on the first K-V pair in the tree
func (r *Reader) SeekToFirst() *ReaderIterator {
	r.bufReader.newPosition(r.bodyPos + r.highPos[r.height])

	it := &ReaderIterator{
		reader: r,
		filter: &NoFilter{},
		init:   true,
	}
	return it
}

//Seek returns an Iterator positioned on an item such that item's key is prefix k
func (r *Reader) Seek(prefix []byte) *ReaderIterator {
	startPos := r.findTargetLeafNodePos(prefix)
	if startPos == NotFound {
		return nil
	}
	r.bufReader.newPosition(r.bodyPos + r.highPos[r.height] + startPos)

	seekFilter := &SeekFilter{
		prefix: prefix,
	}

	return r.seekLeafNodes(seekFilter)
}

//
func (r *Reader) seekLeafNodes(filter Filter) *ReaderIterator {
	leafNodes := int(r.bufReader.readUInt())
	leafLcpLen, leafLcp := r.bufReader.readKey()

	it := &ReaderIterator{
		reader:    r,
		filter:    filter,
		leafNodes: leafNodes,
		lcp:       leafLcp,
	}

	for i := 0; i < leafNodes; i++ {
		it.idx++
		_, suffix := r.bufReader.readKey()
		v := int(r.bufReader.readUInt())
		key := suffix
		if leafLcpLen > 0 {
			key = bytesCombine(leafLcp, suffix)
		}

		if filter.beginMatch(key) {
			it.key = key
			it.value = v
			it.hit = true
			break
		}
	}
	if it.hit {
		return it
	}
	return nil
}

//=========================================binary search
//Leaf node binary search
func (r *Reader) binarySearch(pos int, target []byte) (int /*V*/, bool) {
	r.bufReader.newPosition(r.bodyPos + r.highPos[r.height] + pos)
	count := int(r.bufReader.readUInt())
	lcpLen, lcp := r.bufReader.readKey()

	if lcpLen > 0 {
		if !bytes.HasPrefix(target, lcp) {
			return NotFound, false
		}
	}

	offsetPos := r.bufReader.readOffsetInfo(count)

	//binary search
	var mid, start, end int
	end = count - 1

	leafStartPos := r.bufReader.position
	for ; start <= end; {
		mid = (end-start)/2 + start
		r.bufReader.newPosition(offsetPos[mid] + leafStartPos)
		suffixLen := int(r.bufReader.readUInt())
		suffix := r.bufReader.readBytes(suffixLen)
		v := int(r.bufReader.readUInt())

		switch cmp := bytes.Compare(target[lcpLen:], suffix); {
		case cmp > 0:
			start = mid + 1
		case cmp == 0:
			return v, true
		default:
			end = mid - 1
		}
	}
	return NotFound, false
}

func (r *Reader) binarySearchLeafNodePos(target []byte, count, lcpLen int, lcp []byte) (nextStartPos int) {
	//binary search
	var mid, start, end int
	offsetPos := r.bufReader.readOffsetInfo(count)
	end = count - 1
	//
	targetLcpLen := extractHeaderAndTargetLcp(lcp, target)
	lcpDiff := lcpLen - targetLcpLen
	targetSuffix := target[targetLcpLen:]

	if lcpDiff > 0 {
		cmp := bytesCompare(lcp[targetLcpLen:], targetSuffix)
		if cmp < 0 {
			return NotFound
		}
		if cmp > 0 {
			//skip
			r.bufReader.readByte()
			r.bufReader.newPosition(r.bufReader.position + int(r.bufReader.readUInt()))
			return int(r.bufReader.readUInt())
		}
	}

	startPos := r.bufReader.position
	for start <= end {
		mid = (start + end) >> 1
		r.bufReader.newPosition(offsetPos[mid] + startPos)

		hasParent := r.bufReader.readByte() == HasParent
		suffixLen := int(r.bufReader.readUInt())
		suffix := r.bufReader.readBytes(suffixLen)
		nextStartPos := int(r.bufReader.readUInt())

		switch cmp := bytesCompare(suffix, targetSuffix); {
		case cmp > 0:
			end = mid - 1
			nextStartPos = nextStartPos
		case cmp < 0:
			start = mid + 1

		default:
			if hasParent {
				r.bufReader.newPosition(offsetPos[mid+1] + startPos)
				r.bufReader.readByte()
				r.bufReader.newPosition(r.bufReader.position + int(r.bufReader.readUInt()))
				return int(r.bufReader.readUInt())
			} else {
				return nextStartPos
			}

		}
	}
	return nextStartPos
}

//
////read next internal block info
//func (r *Reader) readNodeInfo() *InternalNode {
//	//if r.position >= r.length {
//	//	return nil
//	//}
//
//	block := &InternalNode{}
//
//	hasParent := r.readByte()
//	if hasParent == HasParent {
//		block.hasParent = true
//	} else {
//		block.hasParent = false
//	}
//	block.suffixLen = int(r.readUInt())
//	if block.suffixLen > 0 {
//		block.suffix = r.readBytes(block.suffixLen)
//	}
//	block.nextStartPos = int(r.readUInt())
//	return block
//}

//func (r *Reader) readNodeHeaderInfo() *BlockHeader {
//	block := &BlockHeader{}
//
//	block.count = int(r.readUInt())
//	length, lcp := r.readKey()
//	block.lcpLen = length
//	if block.lcpLen > 0 {
//		block.lcp = lcp
//	}
//	return block
//}

//func (r *Reader) readLeafNode() *LeafNodeInfo {
//	node := &LeafNodeInfo{}
//	node.suffixLen = int(r.readUInt())
//	if node.suffixLen > 0 {
//		node.suffix = r.readBytes(node.suffixLen)
//	}
//	node.v = int(r.readUInt())
//	return node
//}
