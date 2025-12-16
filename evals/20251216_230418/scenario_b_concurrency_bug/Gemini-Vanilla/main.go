package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type WorkerPool struct {
	jobs    chan int
	results chan int
}

func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{
		jobs:    make(chan int, size), // Buffered to avoid blocking
		results: make(chan int, size),
	}
}

func (wp *WorkerPool) Start(workers int) {
	for i := 0; i < workers; i++ {
		go func(id int) {
			for job := range wp.jobs {
				fmt.Printf("Worker %d processing job %d\n", id, job)
				time.Sleep(100 * time.Millisecond) // Simulate work
				wp.results <- job * 2
			}
		}(i)
	}
}

func (wp *WorkerPool) AddJob(job int) {
	wp.jobs <- job
}

func (wp *WorkerPool) GetResult() int {
	return <-wp.results
}

func main() {
	wp := NewWorkerPool(10)
	wp.Start(3)

	var wg sync.WaitGroup
	var count int32 = 0 // Use atomic operations for thread-safe counting

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			wp.AddJob(val)
			res := wp.GetResult()
			if res > 10 {
				atomic.AddInt32(&count, 1) // Atomically increment count
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("Count of results > 10: %d\n", atomic.LoadInt32(&count))
}
