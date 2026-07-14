package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeCache 是一個並發安全的記憶體快取
type SafeCache struct {
	mu    sync.RWMutex      // 讀寫鎖 (注意：結構體帶有鎖時，傳遞時必須用指標)
	store map[string]string // 底層不安全的 map
}

// NewSafeCache 初始化
func NewSafeCache() *SafeCache {
	return &SafeCache{
		store: make(map[string]string),
	}
}

// Get 讀取資料 (使用 RLock)
func (c *SafeCache) Get(key string) (string, bool) {
	c.mu.RLock()         // 申請「讀鎖」
	defer c.mu.RUnlock() // 確保函式結束時一定會釋放

	val, exists := c.store[key]
	return val, exists
}

// Set 寫入資料 (使用 Lock)
func (c *SafeCache) Set(key, value string) {
	c.mu.Lock()         // 申請「寫鎖」(排他)
	defer c.mu.Unlock() // 確保函式結束時一定會釋放

	c.store[key] = value
	fmt.Printf("[寫入] 已更新 key: %s, value: %s\n", key, value)
}

func main() {
	cache := NewSafeCache()
	cache.Set("version", "v1.0.0")

	var wg sync.WaitGroup

	// 模擬 5 個 Reader (讀取者) 瘋狂讀取
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				val, _ := cache.Get("version")
				fmt.Printf("[讀取] Reader %d 讀到: %s\n", readerID, val)
				time.Sleep(10 * time.Millisecond) // 模擬讀取間隔
			}
		}(i)
	}

	// 模擬 1 個 Writer (寫入者) 在中間突然更新資料
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond) // 等待一下讓 Reader 先跑

		fmt.Println("==== 準備申請寫鎖，準備更新版號 ====")
		cache.Set("version", "v2.0.0")
		fmt.Println("==== 寫鎖已釋放 ====")
	}()

	wg.Wait()
	fmt.Println("所有操作結束！")
}
