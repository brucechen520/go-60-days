package main

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// sync.Once: sync.Once 的 Do 方法確保初始化只會執行一次。它使用一個內部鎖來防止多次調用，並且可以處理併發調用。
var (
	logger    *Logger
	once      sync.Once
	initCount int64
)

type Logger struct {
	conn string
}

func dialLogBackend() *Logger {
	time.Sleep(10 * time.Millisecond)
	atomic.AddInt64(&initCount, 1)
	return &Logger{conn: "log"}
}

func GetLogger() *Logger {
	once.Do(func() {
		logger = dialLogBackend() // 只執行一次
	})
	return logger
}

// sync.Pool
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

const maxBufSize = 64 << 10 // 64KB 上限

func writeLog(msg string) {
	buf := bufPool.Get().(*bytes.Buffer) // 從池中獲取緩衝區
	buf.Reset()
	defer func() {
		if buf.Cap() <= maxBufSize {
			bufPool.Put(buf) // 將緩衝區放回池中
		}

		// cap 太大 -> 不 Put，直接讓 GC 回收那塊大記憶體
	}()

	buf.WriteString(msg)
	_ = buf.String()
}

func main() {
	var wg sync.WaitGroup

	for i := 0; i < 100000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			GetLogger()                             // ★ 併發搶初始化（驗 sync.Once）
			writeLog(fmt.Sprintf("request %d", id)) // 併發用 pool
		}(i)
	}
	wg.Wait()

	got := atomic.LoadInt64(&initCount)
	fmt.Printf("初始化執行次數: %d（應為 1）\n", got)
	if got == 1 {
		fmt.Println("✓ sync.Once：10 萬 goroutine 併發下只初始化一次")
	} else {
		fmt.Printf("✗ 初始化執行了 %d 次\n", got)
	}
}
