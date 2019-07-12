package trie

import "bytes"

const (
	HasParent       = byte('1')
	NoParent        = byte('0')
	HasChildrenNode = byte('1')
	NoChildrenNode  = byte('0')
	NotFound        = -1
	HasValue        = byte('1')
	NoValue         = byte('0')
	SnappyCompress  = byte('1')
	NoCompress      = byte('0')
)

func bytesCombine(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, []byte(""))
}

//Compare the size of two byte arrays
// if a > b return 1
// else if a == b return 0
// if a < b return -1
func bytesCompare(a, b interface{}) int {

	return bytes.Compare(a.([]byte), b.([]byte))
}
