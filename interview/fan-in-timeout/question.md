# 情境題三：跨微服務的資料聚合與超時控制 (Fan-In & Timeout)

## 背景

使用者開「商品詳情頁」時，API Gateway 必須同時向三個微服務請求：
**商品基本資訊**、**使用者評價**、**即時庫存**。

## 流量特徵

- 為保證體驗，API 整體回應時間**不能超過 2 秒**。
- 若「使用者評價」服務異常卡 5 秒，整體 API **不能跟著掛**——2 秒到就直接回傳另外兩個已拿到的資料（partial）。

## 核心考點

1. **Fan-In**：多 goroutine 併發發請求，結果送回同一 channel。
2. **Timeout**：`select` + `context.WithTimeout`（或 `time.After`）做精準超時。
3. **防 Goroutine Leak**：超時後還在背景跑的 goroutine 不能洩漏。

---

## 剖析

### 問題本質
fan-out（打 3 服務）→ fan-in（匯回一處）→ 2 秒硬截止 → 超時回 partial → 背景慢 goroutine 不洩漏。三點環環相扣，**第 3 點（防洩漏）是靈魂**。

### 考點 1：Fan-In
3 個 goroutine 各打一服務，結果送同一個 `results` channel，主線收集。
**關鍵：channel 要 buffered，容量 = goroutine 數**（見考點 3）。

### 考點 2：Timeout
| 方式 | 說明 |
|---|---|
| **`context.WithTimeout`**（推薦）| 慣用法，**還能把取消傳下去**取消底層請求 |
| `time.After(2s)` | 純計時，不會取消下游呼叫 |

收集迴圈用 `select`：一邊收結果、一邊等超時，超時就 break 回 partial。

### 考點 3：防 Goroutine Leak ★

**洩漏怎麼發生**：2 秒到、主線停收 channel。此時評價 goroutine 還在跑（5 秒），最後想 `results <- 結果`——若 channel **unbuffered 且沒人收 → 永遠阻塞 → goroutine 卡死永不退 = 洩漏**。

**兩層防法（都要）**：
1. **buffered channel（容量 = goroutine 數）**：即使沒人收，buffer 有空位，慢 goroutine 送得出去 → 送完就 return → 不阻塞不洩漏。
2. **把 ctx 傳進實際請求**：超時時 ctx 取消 → 底層 HTTP/gRPC 呼叫當場中止（不用跑滿 5 秒）→ goroutine 快速退出、釋放連線。

> 只有 buffered → 不洩漏但 goroutine 空跑 5 秒浪費連線；加 ctx → 2 秒就退。兩個一起才乾淨。

---

## 參考實作骨架

```go
type Result struct {
	Name string
	Data any
	Err  error
}

func aggregate(ctx context.Context) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel() // 確保 ctx 一定被取消，通知所有背景 goroutine 收工

	services := []struct {
		name string
		call func(context.Context) (any, error)
	}{
		{"product", fetchProduct},
		{"reviews", fetchReviews}, // 可能卡 5 秒
		{"stock", fetchStock},
	}

	results := make(chan Result, len(services)) // ★ buffered = goroutine 數

	for _, s := range services {
		go func(name string, call func(context.Context) (any, error)) {
			data, err := call(ctx) // ★ 傳 ctx → 超時時底層請求被取消
			results <- Result{Name: name, Data: data, Err: err} // buffered，永不阻塞
		}(s.name, s.call)
	}

	collected := make(map[string]any)
	for i := 0; i < len(services); i++ {
		select {
		case r := <-results:
			if r.Err == nil {
				collected[r.Name] = r.Data
			}
		case <-ctx.Done(): // ★ 2 秒到 → 回傳已拿到的 partial
			return collected
		}
	}
	return collected
}
```

`time.After` 版本（考點指定要會）：
```go
timeout := time.After(2 * time.Second) // ★ 放迴圈外 = 總截止
for i := 0; i < len(services); i++ {
	select {
	case r := <-results:
		if r.Err == nil { collected[r.Name] = r.Data }
	case <-timeout:
		return collected
	}
}
```

---

## 坑

1. **unbuffered channel → 洩漏**（頭號陷阱）：慢 goroutine 送不出去卡死。容量一定 = goroutine 數。
2. **`time.After` 放進迴圈裡** → 每輪重新計時＝「每輪 2 秒」而非「總共 2 秒」。務必放迴圈外。
3. **沒傳 ctx 給實際請求** → 就算不洩漏，goroutine 也空跑滿 5 秒浪費連線。
4. **忘了 `defer cancel()`** → ctx timer 資源洩漏、也沒通知背景 goroutine。
5. **用 WaitGroup + close(results)** → close 要在另一個 goroutine 等 wg，否則主線 range 跟 wg.Wait 互等死鎖。本題「固定收 N 次」更簡單，不需 close。
6. **partial 要讓上層知道哪個缺** → `collected` 少了 reviews，回應要標記「評價暫時無法載入」，別讓前端以為沒評價。

---

## 執行

```bash
cd interview/fan-in-timeout
go run .              # 模擬 3 服務、reviews 卡 5 秒，驗證 2 秒回 partial
go run -race .        # 驗證無競態
```

用 `runtime.NumGoroutine()` 在超時回傳後、再等一下，證明背景 goroutine 有乾淨退出（無洩漏）。

---

## 面試收束

1. **Fan-In**：N 個 goroutine 送同一個 **buffered channel**（容量 = N）。
2. **Timeout**：`context.WithTimeout` + `select`，`ctx.Done()` 到就回 partial。
3. **防洩漏**：buffered channel（送得出去）+ ctx 傳遞（慢呼叫被取消）→ 背景 goroutine 乾淨退出。
