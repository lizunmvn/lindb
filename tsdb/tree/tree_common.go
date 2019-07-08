package tree

import (
	"bytes"
)

const HasParent = 1
const NoParent = 0

const HasChildrenNode = 1
const NoChildrenNode = 0

const NotFound = -1

//Compare the size of two byte arrays
// if a > b return 1
// else if a == b return 0
// if a < b return -1
func bytesCompare(a, b interface{}) int {
	return bytes.Compare(a.([]byte), b.([]byte))
}

//func stringCompare(a, b interface{}) int {
//	return strings.Compare(a.(string), b.(string))
//}
//
//func intCompare(a, b interface{}) int {
//	return a.(int) - b.(int)
//}

// lcp finds the longest common prefix of the input bytes.
// It compares by bytes instead of runes (Unicode code points).
// It's up to the caller to do Unicode normalization if desired
// (e.g. see golang.org/x/text/unicode/norm).
func lcpByte(l [][]byte) []byte {
	// Special cases first
	switch len(l) {
	case 0:
		return nil
	case 1:
		return l[0]
	}
	// LCP of min and max (lexigraphically)
	// is the LCP of the whole set.
	min, max := l[0], l[0]
	for _, s := range l[1:] {
		switch {
		//bytes.Compare(a.([]byte), b.([]byte))
		case bytes.Compare(s, min) < 0:
			min = s
		case bytes.Compare(s, max) > 0:
			max = s
		}
	}
	for i := 0; i < len(min) && i < len(max); i++ {
		if min[i] != max[i] {
			return min[:i]
		}
	}
	// In the case where lengths are not equal but all bytes
	// are equal, min is the answer ("foo" < "foobar").
	return min
}
