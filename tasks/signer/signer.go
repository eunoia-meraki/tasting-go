package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

func SingleHash(in, out chan interface{}) {

	wg := &sync.WaitGroup{}

	for dataRaw := range in {
		dataInt, ok := dataRaw.(int)
		if !ok {
			log.Println("Single Hash: can't convert data to int")
			continue
		}
		data := fmt.Sprint(dataInt)

		fmt.Print(data, " SingleHash data ", data, "\n")

		md5Data := DataSignerMd5(data)
		fmt.Print(data, " SingleHash md5(data) ", md5Data, "\n")

		wg.Add(1)
		go func(data string, md5Data string, wg *sync.WaitGroup) {
			defer wg.Done()

			crc32DataCh := make(chan string)
			crc32Md5DataCh := make(chan string)

			go func(data string, crc32DataCh chan<- string) {
				crc32Data := DataSignerCrc32(data)
				fmt.Print(data, " SingleHash crc32(data) ", crc32Data, "\n")
				crc32DataCh <- crc32Data
			}(data, crc32DataCh)

			go func(md5Data string, crc32Md5DataCh chan<- string) {
				crc32Md5Data := DataSignerCrc32(md5Data)
				fmt.Print(data, " SingleHash crc32(md5(data)) ", crc32Md5Data, "\n")
				crc32Md5DataCh <- crc32Md5Data
			}(md5Data, crc32Md5DataCh)

			result := <-crc32DataCh + "~" + <-crc32Md5DataCh
			fmt.Print(data, " SingleHash result ", result, "\n")
			out <- result
		}(data, md5Data, wg)
	}
	wg.Wait()
}

func MultiHash(in, out chan interface{}) {

	wgExt := &sync.WaitGroup{}

	for dataRaw := range in {
		data, ok := dataRaw.(string)
		if !ok {
			log.Println("Multi Hash: can't convert data to string")
			continue
		}

		wgExt.Add(1)
		go func(data string, wgExt *sync.WaitGroup) {
			defer wgExt.Done()

			hashes := make(map[int]string)
			mu := &sync.Mutex{}
			wgInt := &sync.WaitGroup{}

			for i := 0; i <= 5; i++ {
				wgInt.Add(1)
				go func(i int, data string, hashes map[int]string, mu *sync.Mutex, wg *sync.WaitGroup) {
					defer wgInt.Done()
					crc32Data := DataSignerCrc32(data)
					fmt.Print(data, " MultiHash: crc32(th+data)) ", i, crc32Data, "\n")
					mu.Lock()
					hashes[i] = crc32Data
					mu.Unlock()
				}(i, fmt.Sprint(i)+data, hashes, mu, wgInt)
			}
			wgInt.Wait()

			result := ""
			for i := 0; i <= 5; i++ {
				result += hashes[i]
			}
			fmt.Print(data, " MultiHash result: ", result, "\n")
			out <- result
		}(data, wgExt)
	}
	wgExt.Wait()
}

func CombineResults(in, out chan interface{}) {

	storage := []string{}

	for dataRaw := range in {
		data, ok := dataRaw.(string)
		if !ok {
			log.Println("CombineResults: can't convert data to string")
			continue
		}
		storage = append(storage, data)
	}

	sort.Strings(storage)

	result := strings.Join(storage, "_")
	fmt.Print("CombineResults ", result, "\n")
	out <- result
}

func ExecutePipeline(jobs ...job) {

	hooks := []chan interface{}{}
	gate := make(chan struct{})

	for _ = range jobs {
		hooks = append(hooks, make(chan interface{}))
	}

	last := len(jobs) - 1

	for i := 0; i < last; i++ {
		go func(i int, in, out chan interface{}) {
			jobs[i](in, out)
			close(out)
		}(i, hooks[i], hooks[i+1])
	}

	go func(i int, in, out chan interface{}) {
		jobs[i](in, out)
		close(out)
		gate <- struct{}{}
		close(gate)
	}(last, hooks[last], hooks[0])

	for _ = range gate {
	}
}
