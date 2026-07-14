package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	totalOrders    = 10000000 // 積壓的訂單數·
	maxConcurrency = 50       // 金流 API 限制：最多同時 50 個連線
)

// Order 一筆待核對的訂單。
type Order struct{ ID int }

// 併發追蹤器：紀錄「同時在飛」的數量與峰值，用來驗證並行真的被限在 50。
var (
	inFlight int64 // 當下同時在跑幾個
	peak     int64 // 歷史峰值
)

// apiLatency 模擬金流 API 的網路延遲。main() 用 200µs 貼近真實；
// benchmark 設 0，才能量到「純協調開銷」（channel/goroutine/lock），不被 sleep 蓋掉。
var apiLatency = 200 * time.Microsecond

// callPaymentAPI 模擬呼叫第三方金流 API (有網路延遲)。
// 進入時 inFlight+1 並更新峰值，離開時 -1。峰值必須 ≤ maxConcurrency 才算限流成功。
func callPaymentAPI(ctx context.Context, o Order) error {
	cur := atomic.AddInt64(&inFlight, 1)
	for { // CAS 更新峰值
		p := atomic.LoadInt64(&peak)
		if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
			break
		}
	}
	defer atomic.AddInt64(&inFlight, -1)

	if apiLatency > 0 {
		time.Sleep(apiLatency) // 模擬 API RTT（benchmark 設 0 略過）
	}
	return nil
}

func resetPeak() { atomic.StoreInt64(&peak, 0); atomic.StoreInt64(&inFlight, 0) }

// ---- Pattern A: Semaphore (每訂單一 goroutine，buffered channel 限並行) ----
func processOrdersSem(orders []Order) {
	sem := make(chan struct{}, maxConcurrency) // 容量 = 最大並行
	var wg sync.WaitGroup

	for _, order := range orders {
		wg.Add(1)         // 派工前 +1
		sem <- struct{}{} //取號：已 50 個在飛就阻塞等 (限流關鍵，放在 go 之前)
		go func(o Order) {
			defer wg.Done()
			defer func() { <-sem }() // 還號，讓下一個進來
			_ = callPaymentAPI(context.Background(), o)
		}(order) // 傳參數，別靠閉包捕獲迴圈變數
	}

	wg.Wait() // 等 1000 萬筆全處理完，main 才安全往下
}

// ---- Pattern B: Worker Pool (固定 50 worker 拉任務) ★ 海量任務首選 ----
func processOrdersPool(orders []Order) {
	jobs := make(chan Order) // 任務 channel (unbuffered，worker 直接拉)
	var wg sync.WaitGroup

	// 固定開 50個 worker：全程只有 50 個 goroutine (不是 1000萬)
	for w := 0; w < maxConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for o := range jobs { //一直拉，直到 jobs 被 close 且清空
				_ = callPaymentAPI(context.Background(), o)
			}
		}()
	}

	// 生產者：把 1000萬筆丟進 channel (50個 worker 都忙時自然阻塞 = backpressure)
	for _, o := range orders {
		jobs <- o
	}

	close(jobs) // 關鍵：通知 worker「沒有更多任務」 ->  for range 結束 -> worker 退出

	wg.Wait() // 等 50個 worker跑完手上最後一筆
}

// ---- 生產版：context 取消 (一處失敗如被鎖 IP -> 中止全部) + error 收集 ----
func processProd(ctx context.Context, orders []Order) error {
	jobs := make(chan Order)
	results := make(chan error, len(orders)) // 帶緩衝，worker 不會卡在寫結果
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for w := 0; w < maxConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for o := range jobs {
				select {
				case <-ctx.Done(): // 已取消 -> 停止拉新任務
					return
				default:
				}
				if err := callPaymentAPI(ctx, o); err != nil {
					results <- err
					cancel() // 一筆失敗 → 取消全部
					return
				}
				results <- nil
			}
		}()
	}

	// 生產者也看 ctx，取消時停止投遞
	// 不能加 default:，如果剛好所有 worker都在忙碌中，生產者不會阻塞，會迅速丟棄後面的 jobs，直到有下一個 worker有空
	go func() {
		defer close(jobs)
		for _, o := range orders {
			select {
			case jobs <- o:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	close(results)

	for err := range results { // 彙整第一個真正的錯
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	orders := make([]Order, totalOrders)
	for i := range orders {
		orders[i] = Order{ID: i}
	}

	run := func(name string, fn func()) {
		resetPeak()
		start := time.Now()
		fn()
		fmt.Printf("[%s] 處理 %d 筆，峰值併發=%d (限制 %d)，耗時 %v\n",
			name, totalOrders, atomic.LoadInt64(&peak), maxConcurrency, time.Since(start).Round(time.Millisecond))
	}

	run("Semaphore", func() { processOrdersSem(orders) })
	run("WorkerPool", func() { processOrdersPool(orders) })
	run("Prod(ctx)", func() { _ = processProd(context.Background(), orders) })

	fmt.Println("重點：三版峰值併發都應 ≤ 50 = 限流成功；go run -race . 驗證無競態")
}
