package main

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

const benchUsers = 1000

// seeded 建一個預填 benchUsers 筆的快取，供壓測用。
func seeded() *PermCache {
	c := New()
	for i := 0; i < benchUsers; i++ {
		c.Set("user"+strconv.Itoa(i), []string{"read", "write"})
	}
	return c
}

// BenchmarkGetParallel：純讀並行。RWMutex 的最佳場景——多讀者用 RLock 完全並行、
// 互不阻塞。這條數字代表「讀多」時的吞吐上限。
func BenchmarkGetParallel(b *testing.B) {
	c := seeded()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get("user" + strconv.Itoa(i%benchUsers))
			i++
		}
	})
}

// BenchmarkReadHeavyParallel：讀多寫少並行（每 1000 次夾 1 次寫），貼近題目流量。
// 罕見的 Lock 會短暫擋住讀者，觀察對吞吐的影響。
func BenchmarkReadHeavyParallel(b *testing.B) {
	c := seeded()
	var writes int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%1000 == 0 {
				c.Set("user"+strconv.Itoa(i%benchUsers), []string{"read"})
				atomic.AddInt64(&writes, 1)
			} else {
				c.Get("user" + strconv.Itoa(i%benchUsers))
			}
			i++
		}
	})
	_ = writes
}

// TestConcurrentNoRace：嚴謹競態壓測。大量併發讀 + 寫 + 整表 Reload，
// 配 `go test -race` 證明 thread-safe（原生 map 併發會 fatal error）。
func TestConcurrentNoRace(t *testing.T) {
	c := seeded()
	var wg sync.WaitGroup
	const readers, writers, iters = 100, 8, 2000

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c.Get("user" + strconv.Itoa((id+i)%benchUsers))
			}
		}(r)
	}
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c.Set("user"+strconv.Itoa((id+i)%benchUsers), []string{"read", "write"})
			}
		}(w)
	}
	// 併發整表 Reload（最激烈：換掉整個 map 指標，跟讀寫互撞）
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			c.Reload(map[string][]string{"userX": {"admin"}})
		}
	}()
	wg.Wait()

	// 最終一致性：Reload 後整表被換掉，舊 key 應消失、新 key 應存在。
	c.Reload(map[string][]string{"final": {"ok"}})
	if p, ok := c.Get("final"); !ok || len(p) != 1 || p[0] != "ok" {
		t.Fatalf("Reload 後狀態不一致：%v ok=%v", p, ok)
	}
	if _, ok := c.Get("user0"); ok {
		t.Fatal("Reload 應清掉舊表，user0 不該還存在")
	}
}

// ---- 版本 B：COW（讀零鎖）壓測，跟版本 A 同條件對比 ----

func seededCOW() *COWCache {
	c := NewCOW()
	for i := 0; i < benchUsers; i++ {
		c.Set("user"+strconv.Itoa(i), []string{"read", "write"})
	}
	return c
}

// BenchmarkCOWGetParallel：純讀並行。讀只有一次 atomic load、零鎖，
// 應明顯快於 RWMutex 版（省掉 RLock 的 atomic + reader 計數開銷）。
func BenchmarkCOWGetParallel(b *testing.B) {
	c := seededCOW()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get("user" + strconv.Itoa(i%benchUsers))
			i++
		}
	})
}

// BenchmarkCOWReadHeavyParallel：讀多寫少並行（每 1000 次夾 1 次寫）。
// 注意：COW 的寫要「複製整表（benchUsers 筆）」，這裡凸顯 COW 的寫成本——
// 表越大、寫越頻繁，這條會越差；讀零鎖的優勢是拿寫的 O(N) 複製換來的。
func BenchmarkCOWReadHeavyParallel(b *testing.B) {
	c := seededCOW()
	var writes int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%1000 == 0 {
				c.Set("user"+strconv.Itoa(i%benchUsers), []string{"read"})
				atomic.AddInt64(&writes, 1)
			} else {
				c.Get("user" + strconv.Itoa(i%benchUsers))
			}
			i++
		}
	})
	_ = writes
}

// TestCOWConcurrentNoRace：COW 併發讀寫 race 壓測。
// COW 併發寫會 lost update（邏輯層面，非 data race），所以這裡只驗「無 race、無 panic」，
// 不做精確計數斷言。配 `go test -race` 證明 atomic.Pointer 讀寫無競態。
func TestCOWConcurrentNoRace(t *testing.T) {
	c := seededCOW()
	var wg sync.WaitGroup
	const readers, writers, iters = 100, 8, 2000

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c.Get("user" + strconv.Itoa((id+i)%benchUsers))
			}
		}(r)
	}
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c.Set("user"+strconv.Itoa((id+i)%benchUsers), []string{"read", "write"})
			}
		}(w)
	}
	wg.Wait()

	// 只驗還讀得到值、程式沒炸（不斷言精確內容，因併發寫會 lost update）
	if _, ok := c.Get("user0"); !ok {
		t.Fatal("user0 應仍存在")
	}
}

// ---- 分級併發壓測：goroutines = 1000 / 5000 / 10000 / 100000 ----
//
// Go benchmark 的 ns/op 就是 RPS 的直接換算：RPS ≈ 1e9 / ns_per_op。
// 例：20 ns/op ≈ 5000 萬 ops/sec。所以「上萬 RPS」對 in-memory 快取毫無壓力，
// 這裡真正要觀察的是「併發量級拉高後，ns/op 會不會因鎖競爭而劣化」。
//
// 用 b.SetParallelism(p) 控制 goroutine 數：實際 goroutine 數 = p × GOMAXPROCS。
// 跑法：go test -bench 'Load' -benchmem -run '^$'

var loadLevels = []int{1000, 5000, 10000, 100000}

// cache 抽象：PermCache（RWMutex）與 COWCache 都滿足，方便同一套壓測跑兩版。
type cache interface {
	Get(string) ([]string, bool)
	Set(string, []string)
}

// benchReadHeavyAt 對指定 cache 跑「讀多寫少（每 1000 次夾 1 寫）」，
// 依 loadLevels 逐級拉高併發 goroutine 數。
func benchReadHeavyAt(b *testing.B, c cache) {
	gomax := runtime.GOMAXPROCS(0)
	for _, n := range loadLevels {
		b.Run(fmt.Sprintf("goroutines=%d", n), func(b *testing.B) {
			p := n / gomax
			if p < 1 {
				p = 1
			}
			b.SetParallelism(p) // 實際 goroutine = p × GOMAXPROCS ≈ n
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					if i%1000 == 0 {
						c.Set("user"+strconv.Itoa(i%benchUsers), []string{"read"})
					} else {
						c.Get("user" + strconv.Itoa(i%benchUsers))
					}
					i++
				}
			})
		})
	}
}

// BenchmarkRWMutexLoad：RWMutex 版逐級加壓。看讀多寫少下 ns/op 隨併發是否穩定。
func BenchmarkRWMutexLoad(b *testing.B) { benchReadHeavyAt(b, seeded()) }

// BenchmarkCOWLoad：COW 版逐級加壓。併發越高、寫者越多 → 複製整表越頻繁，
// 預期 B/op 高且 ns/op 隨併發劣化得比 RWMutex 明顯（COW 寫成本 O(N) 的懲罰）。
func BenchmarkCOWLoad(b *testing.B) { benchReadHeavyAt(b, seededCOW()) }
