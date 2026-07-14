package main

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// 訂單量級：1k / 5k / 10k / 100k / 1M
var benchLevels = []int{1000, 5000, 10000, 100000, 1000000}

func makeOrders(n int) []Order {
	o := make([]Order, n)
	for i := range o {
		o[i] = Order{ID: i}
	}
	return o
}

// benchAllLevels 對某處理函式、指定 API 延遲，逐一量級跑，報 orders/sec 速率。
//   latency = 0        → 量「純協調開銷」（channel/goroutine/lock）
//   latency = 200µs    → 量「API-bound 真實吞吐」，瓶頸在金流 API 而非協調
func benchAllLevels(b *testing.B, latency time.Duration, run func([]Order)) {
	old := apiLatency
	apiLatency = latency
	defer func() { apiLatency = old }()

	for _, n := range benchLevels {
		orders := makeOrders(n) // 預建，不算進計時
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				run(orders)
			}
			b.StopTimer()
			// 處理時間：跑完「一批 n 筆」平均花幾毫秒（越低越快）
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1e6, "ms/batch")
			// 聚合吞吐：總處理筆數 / 總耗時 = orders/sec（越高越好，這就是「RPS」）
			b.ReportMetric(float64(b.N)*float64(n)/b.Elapsed().Seconds(), "orders/sec")
		})
	}
}

const realLatency = 200 * time.Microsecond // 模擬金流 API RTT

// ---- 零延遲：量純協調開銷（三版差異來自 channel/goroutine 成本）----

func BenchmarkSemaphore(b *testing.B) {
	benchAllLevels(b, 0, func(o []Order) { processOrdersSem(o) })
}
func BenchmarkWorkerPool(b *testing.B) {
	benchAllLevels(b, 0, func(o []Order) { processOrdersPool(o) })
}
func BenchmarkProd(b *testing.B) {
	benchAllLevels(b, 0, func(o []Order) { _ = processProd(context.Background(), o) })
}

// ---- 真實 API 延遲（200µs）：量 API-bound 吞吐 ----
// 理論上限 = maxConcurrency / latency = 50 / 200µs = 25 萬 orders/sec。
// 預期三版都被壓到這條線附近 → 證明「瓶頸是 API 延遲，不是 pattern 選型」；
// pattern 差別只在資源（goroutine 數）與功能（取消/收錯），不在吞吐。
// 注意：1M 級在此模式約需 4s/次（1M / 25萬），整組跑約 15~20s。

func BenchmarkSemaphoreAPI(b *testing.B) {
	benchAllLevels(b, realLatency, func(o []Order) { processOrdersSem(o) })
}
func BenchmarkWorkerPoolAPI(b *testing.B) {
	benchAllLevels(b, realLatency, func(o []Order) { processOrdersPool(o) })
}
func BenchmarkProdAPI(b *testing.B) {
	benchAllLevels(b, realLatency, func(o []Order) { _ = processProd(context.Background(), o) })
}
