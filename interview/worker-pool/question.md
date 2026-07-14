# 情境題二：電商大促銷的訂單處理 (Worker Pool & Rate Limiting)

## 背景

雙 11 大促銷期間，系統積壓了 **100,000 筆訂單**，需要非同步呼叫外部「第三方金流 API」核對狀態。

## 流量特徵

- 外部金流 API 承受不住過大流量，嚴格限制「**最多同時 50 個連線**」，超過就**鎖定你的 IP**。
- 既要用 goroutine 盡快消化 10 萬筆，又要嚴格控併發，且 main **必須等全部處理完才能安全關閉**。

## 核心考點

1. **Buffered channel 當信號量（Semaphore）限制最大併發數**
   `sem := make(chan struct{}, 50)`；`sem <- struct{}{}` 取號（滿了阻塞）、`<-sem` 還號。容量 = 最大並行。
2. **`sync.WaitGroup` 等所有非同步任務優雅結束**
   `Add` 在派工前、`Done` 在 defer、`Wait` 在最後。

---

## 兩種實作 pattern

| | Semaphore（A）| Worker Pool（B）|
|---|---|---|
| goroutine 數 | 陸續創 N 個（存活 ≤ 50）| 固定 50 個 |
| 適合 | 任務數不極端、圖簡單 | **海量任務（10 萬筆）** |
| 資源 | 較費 | 省（就 50 個）|
| 限流機制 | channel 容量 | worker 數量 |

→ **10 萬筆選 Worker Pool（B）**：全程只有 50 個 goroutine，不創 10 萬個。

---

## 實作

- `main.go`
  - `processOrdersSem`  — Pattern A：semaphore（每訂單一 goroutine，限並行 50）
  - `processOrdersPool` — Pattern B：worker pool（固定 50 worker 拉任務）★
  - `processProd`       — 生產版：context 取消 + error 收集
  - `main()`            — 三版各跑 10 萬筆，用 atomic 追蹤「同時在飛峰值」驗證 ≤ 50

---

## 坑

1. **閉包捕獲迴圈變數**（Go 1.22 前）：`go func(){ ...order... }()` 全看到最後一筆。修：傳參數 `go func(o Order){}(order)`。（Go 1.22+ 迴圈變數每輪獨立，已安全。）
2. **忘了 `close(jobs)`** → worker `for range` 永遠等 → `wg.Wait()` 死鎖。
3. **worker 裡 `close(jobs)`** → 多 worker 搶 close → `panic: close of closed channel`。**只有生產者能 close**。
4. **results channel 無緩衝 + 沒人讀** → worker 寫 results 卡住 → 死鎖。帶緩衝或另開 goroutine 收。
5. **`wg.Add` 放進 goroutine 裡** → `Wait()` 可能先跑到 → 提早返回。`Add` 一定在 `go` 之前。
6. **sem 取號放 `go` 之後** → 迴圈瞬間創 10 萬 goroutine 才各自搶號 → 沒擋住爆量。取號要在 `go` 之前。

---

## 執行

```bash
cd interview/worker-pool
go run .              # 三版各跑一次，印峰值併發（應 ≤ 50）+ 耗時
go run -race .        # 驗證無競態
go test -bench . -run '^$'   # 不同 worker 數的吞吐對比
```

---

## 面試收束

1. 限並行：buffered channel 信號量（容量 50）或 worker pool（50 worker）。
2. 等結束：`WaitGroup`（Add 派工前 / Done defer / Wait 最後）。
3. 10 萬筆選 worker pool：固定 50 goroutine。
4. `close(jobs)` 通知 worker 收工；只由生產者 close。
5. 進階：`context` 一處失敗取消全部、results channel 收錯。
