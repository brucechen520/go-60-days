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
```

---

## Benchmark

`worker_test.go` 對三版 × 五量級（1k/5k/10k/100k/1M）量測，用 atomic 峰值驗證併發 ≤ 50。
關鍵設計：`apiLatency` 可調——設 0 量「純協調開銷」，設 200µs 量「真實 API-bound 吞吐」。

### 語法

```bash
# 零延遲：量純協調開銷（channel/goroutine 成本），三版差異看這個
go test -bench 'Semaphore$|WorkerPool$|Prod$' -run '^$'

# 真實 API 延遲（200µs）：量吞吐上限，1M 級約 5s/次、整組約 20s
go test -bench 'API' -run '^$' -benchtime 1x -timeout 600s

# 穩定數字（多次取平均，配 benchstat）
go test -bench . -run '^$' -benchtime 5x -count 3 | tee out
benchstat out    # go install golang.org/x/perf/cmd/benchstat@latest
```

benchmark 三招：
- `-run '^$'`：配空 test 名 → 只跑 benchmark、跳過一般 test。
- `-benchtime 1x`：每支只跑 1 次（帶延遲的大量級太慢，用 1x；要穩定改 `5x` + `-count`）。
- `ReportMetric`：自訂欄位。這裡報 `ms/batch`（處理時間）+ `orders/sec`（吞吐/RPS）。

### 結果 A：零延遲（純協調開銷）

| 量級 | Semaphore orders/sec | WorkerPool orders/sec | Prod orders/sec |
|---|---|---|---|
| 1,000 | 547k | 972k | 806k |
| 10,000 | 1.07M | 1.08M | 809k |
| 100,000 | 1.13M | 1.18M | 882k |
| 1,000,000 | 1.34M | 1.18M | 864k |

- **WorkerPool 最穩**（~110 萬/sec，量級拉大不掉）：固定 50 goroutine。
- **Semaphore 小批較慢**：每筆創一個 goroutine，訂單少時創建成本佔比高。
- **Prod 慢 ~25%**：每筆多一次 `results <-` 寫入 + `select` 檢查 ctx（取消/收錯不是免費）。

### 結果 B：真實 API 延遲 200µs（含處理時間）

| 量級 | Semaphore 時間 / RPS | WorkerPool 時間 / RPS | Prod 時間 / RPS |
|---|---|---|---|
| 1,000 | 5.5ms / 181k | 6.0ms / 167k | 5.6ms / 180k |
| 10,000 | 51ms / 195k | 49ms / 204k | 50ms / 200k |
| 100,000 | 495ms / 202k | 476ms / 210k | 535ms / 187k |
| 1,000,000 | 5.56s / 180k | 4.95s / 202k | 5.57s / 179k |

**三版 orders/sec 全擠在 ~18~21 萬**——撞到 API 天花板。

### 核心結論

1. **吞吐由 API 延遲決定，不是 pattern**：理論上限 = `併發數 / 延遲 = 50 / 200µs = 25 萬/sec`，實測 ~20 萬。三版一樣，因為瓶頸是金流 API。
2. **處理時間隨量級線性**：`處理時間 = 量級 / 速率`。10 萬筆 ≈ 0.5s、100 萬筆 ≈ 5s 清完。
3. **零延遲 110 萬 vs API 延遲 20 萬**（差 5 倍）：API 延遲把吞吐從協調上限壓到 API 上限；協調的 1µs 相比 API 的 200µs 可忽略。
4. **要更快只有兩招**：(a) 提高併發上限（50→100，但可能被鎖 IP）(b) 降 API 延遲（快取/批次 API）。**加 goroutine 沒用**——被 50 併發卡死。
5. **選 WorkerPool 不為快**（三版都被 API 卡住一樣），為**省資源**（50 vs 100 萬 goroutine）+ **穩定** + Prod 的**取消/收錯**。

---

## 面試收束

1. 限並行：buffered channel 信號量（容量 50）或 worker pool（50 worker）。
2. 等結束：`WaitGroup`（Add 派工前 / Done defer / Wait 最後）。
3. 10 萬筆選 worker pool：固定 50 goroutine。
4. `close(jobs)` 通知 worker 收工；只由生產者 close。
5. 進階：`context` 一處失敗取消全部、results channel 收錯。
