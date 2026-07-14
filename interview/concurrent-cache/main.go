package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// PermCache 權限白名單快取 (讀多寫少)
type PermCache struct {
	mu    sync.RWMutex
	perms map[string][]string // userID -> 權限清單
}

func New() *PermCache {
	return &PermCache{perms: make(map[string][]string)}
}

// Get 讀取： Rlock允許多個併發讀同時進行
func (c *PermCache) Get(userID string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock() //保證任何 return / panic都釋放讀鎖
	p, ok := c.perms[userID]
	return p, ok
}

// Set寫入：Lock獨佔，寫的當下擋掉所有讀寫。
func (c *PermCache) Set(userID string, perms []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perms[userID] = perms
}

// Reload 幾天一次的整表更新。
func (c *PermCache) Reload(all map[string][]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perms = all
}

// Delete 刪除一個使用者的全部權限。
func (c *PermCache) Delete(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.perms, userID)
}

// COWCache 讀完全無鎖；寫時複製整表原子替換。
type COWCache struct {
	v atomic.Pointer[map[string][]string] // Go1.19+泛型版，型別安全
}

func NewCOW() *COWCache {
	c := &COWCache{}
	m := make(map[string][]string) // 空 map
	c.v.Store(&m)                  // 初始值：指向空 map（nil pointer）
	return c
}

// Get 讀：一次 atomic load, 零鎖。
func (c *COWCache) Get(userID string) ([]string, bool) {
	m := *c.v.Load() // 原子拿當前快照
	p, ok := m[userID]
	return p, ok
}

// Set 寫：複製全表 -> 改副本 -> 原子換指標。寫少所以複製成本可以接受
// 目前寫法，如果遇到併發寫，會發生後寫的會覆蓋前寫的；若有併發寫，寫路徑要再加一把 sync.Mutex(只鎖寫，不影響讀零鎖)
func (c *COWCache) Set(userID string, perms []string) {
	old := *c.v.Load()
	newM := make(map[string][]string, len(old)+1)
	for k, val := range old { // 複製舊表
		newM[k] = val
	}
	newM[userID] = perms // 改副本
	c.v.Store(&newM)     // 原子替換，讀者下次 Load看到新表
}

// ---- 版本 C：分片鎖（sharded lock）----
// map 切成 shardCount 片、各一把 RWMutex。key hash 決定進哪片；讀寫都只鎖「一片」。
// 好處：鎖競爭降到 1/N，不同 key 的寫可並行、寫不複製全表 → 表大/寫也不少的首選。
// 代價：跨片操作（整表 Reload、算全表大小）要鎖全部片，較麻煩。

const shardCount = 256 // 建議 2 的次方，方便用位遮罩取代取模

type shard struct {
	mu sync.RWMutex
	m  map[string][]string
}

type ShardedCache struct {
	shards [shardCount]shard
}

func NewSharded() *ShardedCache {
	c := &ShardedCache{}
	for i := range c.shards {
		c.shards[i].m = make(map[string][]string)
	}
	return c
}

// shard 用 FNV-1a hash 把 key 映射到某一片（reader 計數、鎖競爭都被打散到各片）。
func (c *ShardedCache) shard(key string) *shard {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return &c.shards[h&(shardCount-1)] // shardCount 為 2 次方 → 位遮罩比 % 快
}

func (c *ShardedCache) Get(userID string) ([]string, bool) {
	s := c.shard(userID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.m[userID]
	return p, ok
}

func (c *ShardedCache) Set(userID string, perms []string) {
	s := c.shard(userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[userID] = perms
}

func main() {
	cache := New()
	cache.Set("user1", []string{"read", "write"})
	cache.Set("user2", []string{"read"})

	// WaitGroup：main 必須等所有 goroutine 跑完，否則 main return 會直接結束程式
	var wg sync.WaitGroup

	// 讀多：50 個 goroutine 各讀 20 次（模擬「讀多」的併發讀，全部並行）
	for r := 0; r < 50; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				cache.Get("user1")
			}
		}()
	}

	// 寫少：1 個 goroutine 偶爾寫（寫的當下 Lock 獨佔）
	wg.Add(1)
	go func() {
		defer wg.Done() // 寫者同樣必須 Done，否則 wg 計數不歸零、Wait() 永久阻塞
		for i := 0; i < 10; i++ {
			cache.Set("user1", []string{"read", "write", "delete"})
		}
	}()

	wg.Wait() // 等所有讀者+寫者結束
	fmt.Println("[RWMutex] 完成：50×20 併發讀 + 併發寫，無 fatal error = thread-safe")

	// ---- 版本 B：COW（讀零鎖）----
	cow := NewCOW()
	cow.Set("user1", []string{"read", "write"})

	var wg2 sync.WaitGroup
	// 讀多：讀完全無鎖，只做一次 atomic load
	for r := 0; r < 50; r++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for i := 0; i < 20; i++ {
				cow.Get("user1")
			}
		}()
	}
	// 寫少：COW 併發寫會 lost update（後寫覆蓋前寫），示範用「單一寫者」
	// 若要支援併發寫，Set 內部再加一把 sync.Mutex（只鎖寫、不影響讀零鎖）
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for i := 0; i < 10; i++ {
			cow.Set("user1", []string{"read"})
		}
	}()
	wg2.Wait()
	fmt.Println("[COW] 完成：讀零鎖 + 單寫者，無 fatal error")

	// ---- 版本 C：分片鎖（讀多寫少綜合最優）----
	sharded := NewSharded()
	sharded.Set("user1", []string{"read", "write"})

	var wg3 sync.WaitGroup
	// 讀多：RLock 但打散到 256 片，reader 計數競爭降 1/256
	for r := 0; r < 50; r++ {
		wg3.Add(1)
		go func() {
			defer wg3.Done()
			for i := 0; i < 20; i++ {
				sharded.Get("user1")
			}
		}()
	}
	// 寫：只鎖命中的那一片，其他 255 片照跑；無 lost update（每片獨立鎖精確保護）
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		for i := 0; i < 10; i++ {
			sharded.Set("user1", []string{"read", "write", "delete"})
		}
	}()
	wg3.Wait()
	fmt.Println("[Sharded] 完成：讀打散 + 寫只鎖一片，無 fatal error")

	fmt.Println("驗證競態：go run -race .（三版皆應無 race 警告）")
}
