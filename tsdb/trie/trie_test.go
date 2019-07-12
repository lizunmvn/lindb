package trie

import (
	"fmt"
	"testing"
)

const Record = 1000000

func Test_Write(t *testing.T) {
	trie := NewTrie()

	for i := 0; i < Record; i++ {
		//v := rand.Intn(Record)
		v := i
		trie.put([]byte(fmt.Sprintf("%s%d", "key-", v)), v)
	}
	//trie.put([]byte("key-"), i)

	trie.extractCommonPrefix()
	w := NewWriter(trie)
	bufs := w.encode()
	fmt.Println("file-size:", len(bufs))

	//r := NewReader(bufs)
	//
	//r.Get([]byte("key-100000"))
	//
	//var success = 0
	//for i := 0; i < Record; i++ {
	//	//v := rand.Intn(Record)
	//	v := i
	//	v, ok := r.Get([]byte(fmt.Sprintf("%s%d", "key-", v)))
	//	if !ok {
	//		fmt.Println("error:", i)
	//	} else {
	//		//fmt.Println(i)
	//		success++
	//	}
	//}
	//fmt.Println("success:", success)
}
