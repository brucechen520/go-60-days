# 併發快取選型筆記：RWMutex vs COW vs 分片鎖

> 情境：讀多寫少的 in-process 記憶體快取（權限白名單）。核心是「怎麼在多 goroutine 下安全存取 map，又把讀效能拉到最高」。

---

## 0. 前提：為什麼原生 map 不能直接併發用

Go 的 `map` **故意不做 thread-safe**（省開銷）。runtime 內建偵測：偵到「寫的同時有人讀/寫」→ 直接

```
fatal error: concurrent map read and map write
```

**是 `fatal error`，不是 panic**——`recover()` 救不了，整個 process 死。所以加同步機制不是選配，是必須。

---

## 1. 三種方案總覽

| 方案 | 讀 | 寫 | 一句話 |
|---|---|---|---|
| **`sync.RWMutex`** | 共享（多讀並行） | 獨佔 | 通用預設、面試標準答案 |
| **COW + `atomic.Value`** | **完全無鎖** | 複製整表原子替換 | 極端讀多寫少 + 表不大 |
| **分片鎖（sharded lock）** | 只鎖一片 | 只鎖一片 | 表大 / 寫也不算少 |

---

## 2. `sync.RWMutex`（版本 A）

```go
type PermCache struct {
    mu    sync.RWMutex
    perms map[string][]string
}

func (c *PermCache) Get(id string) ([]string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()   // Lock 下一行立刻 defer，保證釋放
    p, ok := c.perms[id]
    return p, ok
}

func (c *PermCache) Set(id string, perms []string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.perms[id] = perms
}
```

- **RLock 多讀並行**，只有 `Lock`（寫）獨佔 → 讀多寫少遠勝 `Mutex`（Mutex 讀也互斥、浪費並行）。
- **成本**：`RLock`/`RUnlock` 內部仍有 atomic 操作 + reader 計數維護，高頻讀下不為零。
- **寫優先**：一旦有 writer 在等，後來的新讀者也會被擋（防寫飢餓）。

**何時選**：通用場景、寫不算極少、表可能大、要求「讀寫都精確一致」。**不確定就選它**。

---

## 3. COW + `atomic.Value`（版本 B）

讀零鎖；寫時複製整表、原子替換指標。

```go
type COWCache struct {
    v atomic.Pointer[map[string][]string] // Go 1.19+ 泛型版，型別安全
}

func (c *COWCache) Get(id string) ([]string, bool) {
    m := *c.v.Load()       // 一次 atomic load，零鎖
    p, ok := m[id]
    return p, ok
}

func (c *COWCache) Set(id string, perms []string) {
    old := *c.v.Load()
    newM := make(map[string][]string, len(old)+1)
    for k, v := range old { newM[k] = v }  // 複製整表 O(N)
    newM[id] = perms
    c.v.Store(&newM)       // 原子替換，讀者下次 Load 看到新表
}
```

- **讀**：只一次 atomic pointer load，**比 RWMutex 更快**（連 RLock 開銷都省）。
- **讀到一致快照**：要嘛全舊、要嘛全新，不會半舊半新。
- **代價**：每次寫 **O(N) 複製 + 瞬間記憶體翻倍**（新舊表並存到舊表沒人讀）。
- **併發寫會 lost update**：兩個寫者各複製舊表、後 Store 者覆蓋前者。要支援併發寫 → 寫路徑再加一把 `sync.Mutex`（只鎖寫、**不影響讀零鎖**）。

**何時選**：極端讀多寫少（上萬讀 / 幾天一寫）**且表不大**。寫少 → O(N) 複製攤平近乎零；讀零鎖淨賺。

**何時別選**：表大（百萬 key）或寫頻繁（每秒多次）→ 每次複製爆炸 + GC 壓力 + 記憶體抖動 → 改分片鎖。

---

## 4. 分片鎖（sharded lock）

map 切 N 片、各一把 `RWMutex`。key hash 決定進哪片；讀寫都只鎖**一片**。

```go
const shardCount = 256

type ShardedCache struct {
    shards [shardCount]struct {
        mu sync.RWMutex
        m  map[string][]string
    }
}

func (c *ShardedCache) shard(key string) *struct {
    mu sync.RWMutex
    m  map[string][]string
} {
    h := fnv32(key) // 任一 hash
    return &c.shards[h%shardCount]
}

func (c *ShardedCache) Get(key string) ([]string, bool) {
    s := c.shard(key)
    s.mu.RLock()
    defer s.mu.RUnlock()
    p, ok := s.m[key]
    return p, ok
}

func (c *ShardedCache) Set(key string, perms []string) {
    s := c.shard(key)
    s.mu.Lock()
    defer s.mu.Unlock()
    s.m[key] = perms
}
```

- **鎖競爭降到 1/N**：不同片的寫互不阻塞；寫**不複製全表**（只動一片）。
- `sync.Map` 內部就是類似思路（讀路徑用 read-only map 免鎖 + dirty map 補寫）。
- **代價**：實作較複雜；跨片操作（如「整表 Reload」「算全表大小」）要鎖全部片，較麻煩。

**何時選**：表大、寫也不算極少、高併發。**大表首選**。

---

## 5. 選型決策：看「表大小 × 寫頻率」

```
                     寫頻繁 ──────────────▶ 寫極少
        表小  │  RWMutex / 分片鎖        COW ★（讀零鎖淨賺）
        表大  │  分片鎖 ★                分片鎖 或 COW（看寫多稀有）
```

- **不確定 / 通用** → `RWMutex`（安全牌）
- **上萬讀、幾天一寫、表不大**（本題）→ **COW**
- **表大、寫不算少、高併發** → **分片鎖**
- 進階還有：**immutable / persistent 結構**（structural sharing，改一個 key 只複製被動路徑 → O(log N) 而非 O(N)），但 Go 沒內建、要引 lib。

---

## 5.5 讀多 vs 寫多 完整對照

上面全在講**讀多寫少**。面試常反問「那**寫多讀少**呢？」——答案完全不同。

### 為什麼寫多不能用讀多那套

| 方案 | 寫多讀少時 | 為什麼 |
|---|---|---|
| **COW** | ❌ 災難 | 每次寫複製全表 O(N)，寫多 = 一直複製 + GC 壓力爆炸 |
| **RWMutex** | ❌ 退化 | 寫獨佔 → 寫多互相排隊；讀少 → 「多讀並行」優勢用不上；更慘的是為支援讀寫分離要維護 **reader 計數**，讀少時這開銷是**純浪費**，常**比樸素 Mutex 還慢** |

**核心洞見**：`RWMutex` 的價值**來自「讀遠多於寫」**。寫一多它就退化，甚至輸給樸素 `Mutex`。

### 寫多讀少的正解

| 情況 | 用 | 為什麼 |
|---|---|---|
| 只是計數器 / 單一數值 | **`atomic`**（`atomic.AddInt64` 等）| 完全無鎖，最快，寫多也不怕 |
| 一般 map、寫多、key 集中 | **`sync.Mutex`**（不是 RWMutex）| Mutex 更輕，沒有 reader 計數的浪費 |
| map 寫多、key 分散、高併發 | **分片 `Mutex`**（sharded）| 寫只鎖一片，不同 key 的寫**並行**，鎖競爭降 1/N |
| 寫邏輯複雜 / 要嚴格順序 | **channel 序列化到單 goroutine**（actor）| 所有寫送 channel 給單一 owner 處理 → 無鎖、天然順序 |

**別用**：
- `COW`（寫複製爆炸）
- `sync.Map` 於寫多——它是**讀優化**結構（官方文檔：適合「讀多寫少 append-only」或「多 goroutine 操作不相交 key」），寫多且 key 重疊時走 dirty map 有升級成本，**反而慢**

### 一張圖收束（讀多 vs 寫多）

```
讀 >> 寫   → RWMutex（讀並行）；極端 + 表小 → COW（讀零鎖）
寫 >> 讀   → Mutex（樸素就好，RWMutex 的 reader 計數是浪費）；高併發 → 分片 Mutex
純計數     → atomic（無鎖，讀寫多都適用）
要順序     → channel 序列化（actor 模型）
表大高併發 → 分片鎖（不管讀多寫多都能打）
```

> 記法：**RWMutex 挑「讀多」、Mutex 挑「寫多」、atomic 挑「純數值」、分片挑「大表高併發」、channel 挑「要順序」。COW 只在「讀爆多 + 寫爆少 + 表小」這個窄縫最強。**

---

## 6. Benchmark 實測（本專案，benchUsers=1000）

| | ns/op | B/op | 說明 |
|---|---|---|---|
| RWMutex GetParallel | 55 | 2 | 純讀，有 RLock 開銷 |
| **COW GetParallel** | **21** | 2 | 純讀零鎖，快 ~2.6 倍 |
| RWMutex ReadHeavy | 75 | 2 | 讀多寫少 |
| **COW ReadHeavy** | 61 | **101** | 較快但記憶體暴增 |

**關鍵**：COW ReadHeavy 的 `B/op` 從 2 飆到 **101** = 「寫複製全表」成本現形。表越大這數字越爆。把 `benchUsers` 改 100000 再跑：`COWGetParallel` 不變（讀不複製）、`COWReadHeavy` 的 B/op 噴天 → 一眼看出「讀零鎖不受表大小影響，寫複製受表大小懲罰」。

跑法：
```bash
go test -bench . -benchmem -run '^$'   # A vs B 對比
go test -race -run NoRace              # thread-safe 驗證
```

### 6.1 分級併發壓測（goroutines = 1000 / 5000 / 10000 / 100000）

先記一條換算：**Go benchmark 的 `ns/op` 就是 RPS 的直接代理 → `RPS ≈ 1e9 / ns_per_op`**。20 ns/op ≈ 5000 萬 ops/sec。所以「上萬 RPS」對 in-memory 快取差兩三個數量級、毫無壓力。分級壓測真正要看的是**併發拉高後 ns/op 會不會因鎖競爭劣化**。

用 `b.SetParallelism(p)` 控制 goroutine 數（實際 = `p × GOMAXPROCS`），讀多寫少（1/1000 寫）逐級加壓：

| 併發 | RWMutex ns/op | RWMutex B/op | COW ns/op | COW B/op |
|---|---|---|---|---|
| 1000 | 180 | 15 | **80** | 118 |
| 5000 | 189 | 15 | 124 | 118 |
| 10000 | 248 | 14 | 158 | 119 |
| 100000 | 197 | 15 | 117 | 157 |

**發現（趨勢跟低併發打平時反轉）**：

1. **RPS 面**：全部 < 250 ns/op → 聚合 400 萬+ ops/sec。上萬 RPS 根本不是問題。

2. **高併發下 COW 的 ns/op 反而更低（80~158 vs 180~248）**——因為 **RWMutex 的 reader 計數變成競爭點**：每個 `RLock` 要對同一個 reader counter 做 atomic 加減 → 上萬 goroutine 在多核間狂搶同一條 cache line（cache-line bouncing）→ 讀延遲被拖高。COW 的讀只有 **atomic pointer LOAD**（純讀、不寫、無計數器）→ cache line 保持共享態，**併發越高越顯出零鎖讀的擴展性**。

3. **COW 的記憶體稅一直在（B/op 15 vs 118~157）**：多出的 ~100 B/op 是「寫複製整表」成本攤到讀操作上。RWMutex 穩定 15 B/op（那 15 是 `strconv.Itoa` baseline，不是鎖）。

**心智模型更新**：
```
低併發 + 有寫  → RWMutex、COW 的 ns/op 接近
高併發 + 讀多  → COW 讀零鎖「擴展性」勝出（RWMutex reader 計數變競爭點）
             但 COW 一律付「寫複製全表」的記憶體稅（B/op 高 8~10 倍）
```
→ 極端讀多 + 高併發選 COW，不只是單次讀快，是**併發擴展性**好（無 reader 計數競爭）；代價永遠是寫的記憶體。

跑法 + 注意：
```bash
go test -bench Load -benchmem -run '^$'          # 分級壓測
go test -bench Load -benchmem -count 5 | tee out  # 多次取樣
benchstat out                                     # 平均 + 標準差（go install golang.org/x/perf/cmd/benchstat@latest）
```
> 100000 goroutine 是極端值，Go 排程開銷開始參雜、數字會抖（100000 有時比 10000 低就是排程平滑化）。要穩定對比用 `-count` + `benchstat`。

---

## 7. `defer` + 死鎖坑（三種方案共通）

**黃金公式**：`Lock()` 下一行**立刻** `defer Unlock()`。保證任何 return / panic 都釋放。

### 坑 1：重入（reentrant）— Go 的鎖不可重入

**重入 = 同一 goroutine 已持鎖，又想再拿同一把鎖。** Go 的 `Mutex`/`RWMutex` 都**不可重入** → 第二次 Lock 死鎖（自己等自己放）。

```go
func (c *Cache) A() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.B()          // ✗ B 也 Lock 同一把 → deadlock
}
func (c *Cache) B() {
    c.mu.Lock()    // 死鎖：A 還持著，B 等，但 B 就是 A 這條 goroutine
    defer c.mu.Unlock()
}
```

**破解：`xxxLocked` 慣例**——公開方法加鎖，內部邏輯抽成不加鎖的私有方法（假設呼叫者已持鎖）：

```go
func (c *Cache) Get(k string) ([]string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.getLocked(k)   // 呼叫不加鎖版
}
func (c *Cache) getLocked(k string) ([]string, bool) { // 假設已持鎖，自己不碰鎖
    p, ok := c.perms[k]
    return p, ok
}
```

**RWMutex 特有重入坑**：持 `RLock` 時再 `RLock` 同一把——看似讀共享 OK，但若中間有 writer 在等，寫優先擋第二個 RLock、writer 又等第一個 RLock 放 → 卡死。官方文檔明確警告別遞迴 RLock。

### 坑 2：臨界區呼叫會加同鎖的函式 → 同上死鎖。臨界區只做純資料操作，別呼叫會再碰鎖的東西。

### 坑 3：`defer` 拉長持鎖 → `defer Unlock` 是函式結束才執行。若鎖後面還做慢事（IO/RPC），鎖被持有整段時間。把慢事移到鎖外，或用小函式縮小臨界區。

### 配對別錯：`RLock` 配 `RUnlock`、`Lock` 配 `Unlock`。

---

## 8. 面試收束（一分鐘版）

1. 原生 map 併發 = `fatal error`（不是 panic、recover 救不了）→ 必須同步。
2. 讀多寫少 → `RWMutex`（讀並行、寫獨佔），勝 `Mutex`（讀也互斥）。
3. **極端**讀多寫少 + 表不大 → **COW + atomic.Value 讀零鎖**，實測純讀快 ~2.6 倍；代價是寫複製全表 O(N) + 記憶體翻倍。
4. **表大 / 寫不算少** → **分片鎖**，鎖競爭降 1/N、寫不複製全表。
5. `defer` 緊接 Lock；Go 鎖**不可重入**，用 `xxxLocked` 私有方法破解。
6. 這是 in-process thread-safe，**不需要 Redis**（Redis 分散式鎖是跨進程、另一層級）。
