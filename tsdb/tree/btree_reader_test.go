package tree

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
	"unsafe"
)

const Record = 1000000

func Test_Reader(t *testing.T) {
	r := newTree()
	testTreeMemGet(r)
	testTreeMemSeek(r)

	encoder := NewEncoder(r)
	bufs := encoder.encode()
	fmt.Println("file-size:", len(bufs))

	reader := NewReader(bufs)
	//
	testTreeReaderGet(reader)
	//testTreeReaderSeek(reader)

	//v, _ := reader.Get([]byte("key-4"))
	//fmt.Println(v)

	//it := reader.Seek([]byte("key-3"))
	//it.Next()
	//for ; it.HasValue(); {
	//	fmt.Println("k:", string(it.GetKey()), " v:", it.GetValue())
	//	it.Next()
	//}
}

func Test_SeeKFirst(t *testing.T) {

	r := newTree()

	encoder := NewEncoder(r)
	bufs := encoder.encode()

	reader := NewReader(bufs)

	it := reader.SeekToFirst()
	success := 0
	for ; it.Next(); {
		success++
	}
	fmt.Println("success: ", success)
}

func Test_Range(t *testing.T) {

	r := newTree()

	encoder := NewEncoder(r)
	bufs := encoder.encode()

	reader := NewReader(bufs)

	it := reader.Range([]byte("key-1000000"), []byte("key-200000"))
	success := 0
	for ; it.Next(); {
		success++
	}
	fmt.Println("success: ", success)

}

func testTreeReaderSeek(reader *Reader) {
	success := 0
	startTime := time.Now().UnixNano()
	for i := 0; i < 10; i++ {
		success = 0
		key := fmt.Sprintf("%s%d", "key-", i)
		it := reader.Seek([]byte(key))
		if nil != it {
			for ; it.Next(); {
				success++
				//fmt.Println("k:", string(it.GetKey()), " v:", it.GetValue())
			}
		}
		//fmt.Println(i, " seek-success:", success)
	}
	fmt.Println("file-seek:", (time.Now().UnixNano()-startTime)/1000000, "ms")
}

func newTree() *Tree {
	t := TreeNew(bytesCompare)

	for i := 0; i < Record; i++ {
		t.Set([]byte(fmt.Sprintf("%s%d", "key-", i)), i)
	}
	return t
}

func testTreeMemGet(t *Tree) {
	success := 0
	startTime := time.Now().UnixNano()
	for i := 0; i < Record; i++ {
		key := fmt.Sprintf("%s%d", "key-", i)
		v, _ := t.Get([]byte(key))
		if i == v {
			success++
		} else {
			fmt.Println("error...", i)
		}
	}
	fmt.Println("mem-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
	fmt.Println("mem-success:", success)
}

func testTreeMemSeek(t *Tree) {
	success := 0
	startTime := time.Now().UnixNano()
	for i := 0; i < 10; i++ {
		success = 0
		key := fmt.Sprintf("%s%d", "key-", i)
		e, ok := t.Seek([]byte(key))
		if ok {
			for {
				_, _, err := e.Next()
				if nil == err {
					success++
				} else {
					break
				}
			}
		}
		//fmt.Println(i, " seek-success:", success)
	}
	fmt.Println("mem-seek:", (time.Now().UnixNano()-startTime)/1000000, "ms")
}

func testTreeReaderGet(r *Reader) {
	success := 0
	startTime := time.Now().UnixNano()
	for i := 0; i < Record; i++ {
		key := fmt.Sprintf("%s%d", "key-", i)
		v, _ := r.Get([]byte(key))
		if i == v {
			success++
		} else {
			fmt.Println("error...", i)
		}
	}
	fmt.Println("file-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
	fmt.Println("file-success:", success)
}

func Test_MultiGet(t *testing.T) {

	//r := newTree()
	//encoder := NewEncoder(r)
	//bufs := encoder.encode()
	//reader := NewReader(bufs)

	//reader.MultiGet([]byte("key-10000"), []byte("key-100"), []byte("key-5000"), []byte("key-100021"))

	//startTime := time.Now().UnixNano()
	//success := 0
	//for i := 0; i < Record; i++ {
	//	key := fmt.Sprintf("%s%d", "key-", i)
	//	v, _ := reader.Get([]byte(key))
	//	if i == v {
	//		success++
	//	} else {
	//		fmt.Println("error...", i)
	//	}
	//}
	//fmt.Println("file-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
	//fmt.Println("file-success:", success)
}

func Test_Random(t *testing.T) {
	r := TreeNew(bytesCompare)
	var records = make(map[string]int)
	for i := 0; i < Record; i++ {
		key := RandStringBytesMaskImprSrc(8)
		records[key] = i
		r.Set([]byte(key), i)
	}

	encoder := NewEncoder(r)
	reader := NewReader(encoder.encode())

	fmt.Println(reader.bodyPos)
	//
	//startTime := time.Now().UnixNano()
	//success := 0
	//for k, v := range records {
	//	value, _ := reader.Get([]byte(k))
	//	if value == v {
	//		success++
	//	} else {
	//		fmt.Println("xx", k)
	//	}
	//}
	//fmt.Println("file-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
	//fmt.Println("file-success:", success)
}

//func printTree(r *Reader) {
//	for h := 1; h <= r.height+1; h++ {
//		r.newPosition(r.highPos[h] + r.bodyPos)
//		nodes := int(r.readUInt())
//
//		var nIndex int
//		for i := 0; i < nodes; i++ {
//			if i < r.height-1 {
//
//				if nIndex == 0 {
//					commonPrefixLen := r.readUInt()
//					if commonPrefixLen > 0 {
//						commonPrefix := r.readBytes(int(commonPrefixLen))
//						fmt.Println("common-prefix....", string(commonPrefix))
//					}
//				}
//
//				var suffixBytes []byte
//				var start int
//				//hasParent := r.readUInt()
//				//todo ??
//				//if hasParent == HasParent {
//				//} else {
//				//}
//				suffixLen := r.readUInt()
//				if suffixLen > 0 {
//					suffixBytes = r.readBytes(int(suffixLen))
//				}
//				start = int(r.readUInt())
//
//				fmt.Println("...treeHigh...", i, "===internal===suffix:", string(suffixBytes), " start:", start)
//				nIndex++
//			} else {
//				var lIndex uint64
//				//if lIndex == 0 {
//				//	leafNodes = r.readUInt()
//				//	fmt.Println("...current..leaf...nodes...", leafNodes)
//				//}
//				//if !(lIndex < leafNodes) {
//				//	leafNodes = r.readUInt()
//				//	fmt.Println("...current..leaf...nodes...", leafNodes)
//				//	lIndex = 0
//				//}
//
//				if lIndex == 0 {
//					commonPrefixLen := r.readUInt()
//					if commonPrefixLen > 0 {
//						commonPrefix := r.readBytes(int(commonPrefixLen))
//						fmt.Println("leaf-common-prefix....", string(commonPrefix))
//					}
//					dataOffset := int(r.readUInt())
//					r.newPosition(r.position + dataOffset)
//				}
//
//				kLen := int(r.readUInt())
//				var key []byte
//				if kLen > 0 {
//					key = r.readBytes(kLen)
//				}
//				v := r.readUInt()
//				fmt.Println("===leaf===", string(key), " value:", v)
//
//				lIndex++
//			}
//		}
//	}
//}

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxMask = 1<<6 - 1 // All 1-bits, as many as 6
)

var src = rand.NewSource(time.Now().UnixNano())

func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for 10 characters!
	for i, cache, remain := n-1, src.Int63(), 10; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), 10
		}
		b[i] = letterBytes[int(cache&letterIdxMask)%len(letterBytes)]
		i--
		cache >>= 6
		remain--
	}
	return *(*string)(unsafe.Pointer(&b))
}

func TestCompare(t *testing.T) {
	b := []byte("xx")
	fmt.Println(bytesCompare(nil, b) > 0)

}
