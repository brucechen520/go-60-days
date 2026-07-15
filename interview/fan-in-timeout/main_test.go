package main

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// assertNoLeak 輪詢等背景 goroutine 退場，最多等 ~1 秒；仍高於 base 才判洩漏。
// （比單次量 NumGoroutine 穩——避免撞到 goroutine 正在退場的瞬間誤判。）
func assertNoLeak(t *testing.T, base int) {
	t.Helper()
	for i := 0; i < 20; i++ {
		if runtime.NumGoroutine() <= base {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("goroutine 洩漏：base=%d，最後=%d", base, runtime.NumGoroutine())
}

// 超時回傳 partial：reviews 卡 5 秒 → 2 秒截止 → 只拿到 product + stock。
func TestAggregate_ReturnsPartialOnTimeout(t *testing.T) {
	got := aggregate(context.Background())

	if _, ok := got["product"]; !ok {
		t.Error("product 該拿到（300ms 內完成）")
	}
	if _, ok := got["stock"]; !ok {
		t.Error("stock 該拿到（500ms 內完成）")
	}
	if _, ok := got["reviews"]; ok {
		t.Error("reviews 該超時、不該出現在 partial 裡")
	}
}

// 2 秒截止生效：不傻等 reviews 的 5 秒。
func TestAggregate_FinishesWithinBudget(t *testing.T) {
	start := time.Now()
	aggregate(context.Background())
	elapsed := time.Since(start)

	if elapsed > 2500*time.Millisecond {
		t.Errorf("應 ~2s 內結束，卻花了 %v（超時控制失效，可能在死等 reviews）", elapsed)
	}
	if elapsed < 1500*time.Millisecond {
		t.Errorf("太快結束 %v，2 秒截止可能沒生效", elapsed)
	}
}

// 無 goroutine 洩漏：超時後背景的 reviews goroutine 要乾淨退出。
func TestAggregate_NoGoroutineLeak(t *testing.T) {
	base := runtime.NumGoroutine()
	aggregate(context.Background())
	assertNoLeak(t, base)
}
