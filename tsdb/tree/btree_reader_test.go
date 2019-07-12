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
	r := TreeNew(bytesCompare)

	for i := 0; i < Record; i++ {
		r.Set([]byte(fmt.Sprintf("%s%d", "key-", i)), i)
	}

	startTime := time.Now().UnixNano()
	success := 0
	for i := 0; i < Record; i++ {
		key := fmt.Sprintf("%s%d", "key-", i)
		v, _ := r.Get([]byte(key))
		if i == v {
			success++
		} else {
			fmt.Println("error...", i)
		}
	}
	fmt.Println("mem-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
	fmt.Println("mem-success:", success)

	encoder := NewEncoder(r)
	bufs := encoder.encode()

	fmt.Println("file-size:", len(bufs))

	//reader := NewReader(bufs)
	////printTree(reader)
	//
	//v, _ := reader.Get([]byte("key-12"))
	//fmt.Println(v)
	//
	//startTime = time.Now().UnixNano()
	//success = 0
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

	//reader.seek([]byte("key-1"))
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

	startTime := time.Now().UnixNano()
	success := 0
	for k, v := range records {
		value, _ := reader.Get([]byte(k))
		if value == v {
			success++
		} else {
			fmt.Println("xx", k)
		}
	}
	fmt.Println("success:", success)
	fmt.Println("task:", (time.Now().UnixNano()-startTime)/1000000, "ms")
}

func printTree(r *Reader) {
	for h := 1; h <= r.high+1; h++ {
		r.newPosition(r.levelPos[h] + r.bodyPos)
		nodes := int(r.readUInt())

		var nIndex int
		for i := 0; i < nodes; i++ {
			if i < r.high-1 {

				if nIndex == 0 {
					commonPrefixLen := r.readUInt()
					if commonPrefixLen > 0 {
						commonPrefix := r.readBytes(int(commonPrefixLen))
						fmt.Println("common-prefix....", string(commonPrefix))
					}
				}

				var suffixBytes []byte
				var start int
				//hasParent := r.readUInt()
				//todo ??
				//if hasParent == HasParent {
				//} else {
				//}
				suffixLen := r.readUInt()
				if suffixLen > 0 {
					suffixBytes = r.readBytes(int(suffixLen))
				}
				start = int(r.readUInt())

				fmt.Println("...treeHigh...", i, "===internal===suffix:", string(suffixBytes), " start:", start)
				nIndex++
			} else {
				var lIndex uint64
				//if lIndex == 0 {
				//	leafNodes = r.readUInt()
				//	fmt.Println("...current..leaf...nodes...", leafNodes)
				//}
				//if !(lIndex < leafNodes) {
				//	leafNodes = r.readUInt()
				//	fmt.Println("...current..leaf...nodes...", leafNodes)
				//	lIndex = 0
				//}

				if lIndex == 0 {
					commonPrefixLen := r.readUInt()
					if commonPrefixLen > 0 {
						commonPrefix := r.readBytes(int(commonPrefixLen))
						fmt.Println("leaf-common-prefix....", string(commonPrefix))
					}
					dataOffset := int(r.readUInt())
					r.newPosition(r.position + dataOffset)
				}

				kLen := int(r.readUInt())
				var key []byte
				if kLen > 0 {
					key = r.readBytes(kLen)
				}
				v := r.readUInt()
				fmt.Println("===leaf===", string(key), " value:", v)

				lIndex++
			}
		}
	}
}

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
