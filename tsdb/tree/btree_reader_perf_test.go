package tree

import (
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

func Test_Profile(t *testing.T) {
	r := TreeNew(bytesCompare)

	for i := 0; i < Record; i++ {
		r.Set([]byte(fmt.Sprintf("%s%d", "key-", i)), i)
	}

	encoder := NewEncoder(r)
	bufs := encoder.encode()

	fmt.Println("file-size:", len(bufs))

	reader := NewReader(bufs)

	go func() {
		time.Sleep(5 * time.Second)
		//defer profile.Start().Stop()
		//defer profile.Start(profile.MemProfile, profile.CPUProfile).Stop()
		//runtime.StartCPUPro
		f, _ := os.Create("test.prof")
		pprof.StartCPUProfile(f)

		fmt.Println("starting...")
		for j := 0; j < 20; j++ {
			startTime := time.Now().UnixNano()
			success := 0
			for i := 0; i < Record; i++ {
				key := fmt.Sprintf("%s%d", "key-", i)
				v, _ := reader.Get([]byte(key))
				if i == v {
					success++
				} else {
					fmt.Println("error...", i)
				}
			}
			fmt.Println(success)
			fmt.Println("file-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
		}
		pprof.StopCPUProfile()
		fmt.Println("close...")
	}()

	http.ListenAndServe("0.0.0.0:6060", nil)

}


func Test_Random_PProf(t *testing.T) {

	r := TreeNew(bytesCompare)

	var records = make(map[string]int)
	for i := 0; i < Record; i++ {
		key := RandStringBytesMaskImprSrc(10)
		records[key] = i
		r.Set([]byte(key), i)
	}

	startTime := time.Now().UnixNano()
	success := 0
	for k, v := range records {
		value, _ := r.Get([]byte(k))
		if value == v {
			success++
		} else {
			fmt.Println("xx", k)
		}
	}
	fmt.Println("mem-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
	fmt.Println(success)

	encoder := NewEncoder(r)
	bufs := encoder.encode()

	fmt.Println("file-size:", len(bufs))
	reader := NewReader(bufs)

	go func() {
		time.Sleep(5 * time.Second)
		//defer profile.Start().Stop()
		//defer profile.Start(profile.MemProfile, profile.CPUProfile).Stop()
		//runtime.StartCPUPro
		f, _ := os.Create("random_mem.prof")
		//pprof.StartCPUProfile(f)
		pprof.WriteHeapProfile(f)

		fmt.Println("starting...")
		for j := 0; j < 10; j++ {
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
			fmt.Println(success)
			fmt.Println("file-get:", (time.Now().UnixNano()-startTime)/1000000, "ms")
		}
		//pprof.StopCPUProfile()
		fmt.Println("close...")
	}()

	http.ListenAndServe("0.0.0.0:6060", nil)

}