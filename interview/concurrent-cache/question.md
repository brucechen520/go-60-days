# 情境題一：高頻讀取的本地快取系統 (Local Cache)

## 背景

實作一個全域的記憶體快取（以 `map` 結構為主），儲存使用者的**權限白名單**。

## 流量特徵

- 系統每秒高達**上萬次「讀取」**請求。
- 權限資料可能**好幾天才「更新（寫入）」一次**。
- → 典型的**讀多寫少**。

## 挑戰

Golang 原生 `map` 遇到併發讀寫會直接產生 `fatal error: concurrent map read and map write`，**整個 process 崩潰**（注意：是 `fatal error`，`recover()` 也救不了）。

## 核心考點

1. **如何保證資料結構的 Thread-safe？**
   原生 map 併發不安全 → 必須加同步機制（`sync.RWMutex` / `sync.Map` / COW + `atomic.Value`）。

2. **讀多寫少該選 `sync.Mutex` 還是 `sync.RWMutex`？**
   選 `RWMutex`：`RLock` 允許多讀並行，`Lock` 寫時獨佔。`Mutex` 讀也互斥，浪費並行度。
   極端讀多寫少的進階答案：COW + `atomic.Value`，讀完全無鎖。

3. **如何正確用 `defer` 保證鎖一定釋放、避免死鎖 (Deadlock)？**
   `Lock()` 下一行立刻 `defer Unlock()`。注意坑：RWMutex 不可重入、臨界區別呼叫會加同鎖的函式、別讓 `defer` 拉長持鎖。

---

## 實作

- `main.go` — 版本 A：`sync.RWMutex` 版 `PermCache`（Get/Set/Reload/Delete）+ 併發讀寫 demo。
- `cache_test.go` — 嚴謹壓測：race 測試 + benchmark。

---

## 執行測試

```bash
cd interview/concurrent-cache
```

### 1. Race 壓測（證明 thread-safe）

```bash
go test -race -run TestConcurrentNoRace -v
```

- `-race`：開競態偵測器（**關鍵**，證明無併發 bug）。
- `-run <正則>`：只跑名字符合的 test。
- `-v`：印每個 test 的 RUN/PASS。
- 有 DATA RACE 會印紅字堆疊；乾淨則 `PASS`。

### 2. Benchmark（測效能）

```bash
go test -bench . -benchmem -run '^$'
```

- `-bench .`：跑所有 `Benchmark*`（`.` 是正則配全部）。
- `-benchmem`：多印記憶體（B/op、allocs/op）。
- `-run '^$'`：配空 test 名 → **跳過一般 test**，只跑 benchmark。

### 3. 直接跑 demo

```bash
go run -race .    # 跑 main()，-race 順便驗證併發安全
```

### 常用加料

```bash
go test -bench . -benchtime 3s      # 每支 benchmark 跑久一點更穩
go test -bench . -cpu 1,2,4,8       # 固定 CPU 數看多核擴展性
go test -bench . -count 5           # 跑多次取平均（配 benchstat 比較版本）
```

### 讀 benchmark 輸出

```
BenchmarkGetParallel-8    14845318    90.96 ns/op    2 B/op    0 allocs/op
        │            │         │          │            │          └ 每次操作分配幾次
        │            │         │          │            └ 每次操作分配幾 byte
        │            │         │          └ 每次操作幾奈秒（越小越快）
        │            │         └ 總共跑幾次（go 自動決定）
        │            └ GOMAXPROCS=8（8 核並行）
        └ benchmark 名
```

> 命名規則：test 函式 `func TestXxx(t *testing.T)`、benchmark `func BenchmarkXxx(b *testing.B)`，檔名必須以 `_test.go` 結尾，`go test` 才認得。
