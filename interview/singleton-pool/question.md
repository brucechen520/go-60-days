# 情境題四：高併發日誌系統的底層優化 (Singleton & Memory Pool)

## 背景

開發一套高效能 HTTP 伺服器框架，每個請求進來都會寫一筆 Log。

## 流量特徵

- 系統**啟動瞬間**湧入巨大流量。
- 高頻率的字串拼接與 Log 格式化 → 產生大量**短暫的 `[]byte`**。

## 挑戰

1. 瞬間高併發 → Log 連線的**初始化動作被觸發多次**（該只做一次）。
2. 大量廢棄的 `[]byte` → **GC 瘋狂運作**、拖垮 CPU。

## 核心考點

1. **`sync.Once`**：保證 Log 元件的初始化，在再高併發下也**絕對只執行一次**（Singleton）。
2. **`sync.Pool`**：物件池，重用 `[]byte`／`bytes.Buffer`，把記憶體分配次數降到最低。

---

## 剖析

兩個**獨立**問題，各對應一個工具，別混。

### 考點 1：sync.Once（初始化只做一次）

**問題**：啟動瞬間上萬 goroutine 同時要寫 log → 都發現「連線還沒建」→ 各自去初始化 → 連線被建 N 次（浪費資源 + race）。

**為什麼不用手動判斷**：
```go
if logger == nil {        // ✗ TOCTOU race
    logger = initLogger() //   兩個 goroutine 同時看到 nil → 都初始化
}
```
兩個 goroutine 同時通過 `== nil`，就初始化兩次。

**為什麼不用 Mutex**：能解，但初始化**完成之後**每次呼叫還是要搶鎖檢查 → 浪費（熱路徑不該一直鎖）。

**sync.Once**：`once.Do(fn)` 保證 `fn` **絕對只跑一次**：
- 第一個呼叫者跑 `fn`，其餘呼叫者**阻塞等它跑完**
- 跑完後，之後的呼叫走 **atomic 快速路徑**（一個 atomic load），幾乎零成本
- 內部：`done` atomic 旗標 + 慢路徑用 mutex；旗標一設，後續全走快路徑

### 考點 2：sync.Pool（重用物件減 GC）

**問題**：每次 log 格式化配一個 `bytes.Buffer`/`[]byte`，用一下就丟 → 高 QPS 下每秒百萬個短命物件 → GC 一直回收 → CPU 被 GC 吃掉。

**sync.Pool**：物件池。`Get()` 拿一個（空池就用 `New` 造）、`Put()` 用完歸還給下次重用 → **分配次數大降** → GC 壓力降。

---

## 參考骨架

### sync.Once（Singleton logger）

```go
var (
	logger *Logger
	once   sync.Once
)

func GetLogger() *Logger {
	once.Do(func() {
		logger = &Logger{conn: dialLogBackend()} // ★ 絕對只執行一次
	})
	return logger
}
```

### sync.Pool（buffer 池）

```go
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) }, // 空池時怎麼造
}

func writeLog(msg string) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()            // ★ 拿到的可能是別人用過的，先清空殘留
	defer bufPool.Put(buf) // ★ 用完歸還

	buf.WriteString(msg)
	// ... 把 buf 內容寫出去（在 Put 前用完）
}
```

---

## 坑

### sync.Once
1. **`Do` 的 fn panic 也算「做過了」**：Once 會認定完成、不重試。初始化可能失敗時要另設計（如帶 error 的 once）。
2. **不能複製**：`sync.Once` 有內部狀態，用過後別 copy（複製會得到「未執行」的假象）。
3. **once 要共享**：得是 package 級或共享實例，不能每次呼叫 new 一個新的 once。

### sync.Pool（坑特別多）
1. **拿到一定要 Reset/清空**：pool 給的是別人用過的物件，有殘留資料 → 不清就是髒資料 bug。
2. **`Put` 之後別再碰那個物件**：歸還後它可能立刻被別的 goroutine `Get` 走 → 你再寫 = data race / 污染。
3. **Pool 內容隨時被 GC 清掉**：sync.Pool 是「減輕 GC」不是「永久快取」——GC 時池內物件可能被清空。所以**別拿它當一定拿得到的快取**，也**別把需要顯式關閉的資源（DB 連線）放進去**期望常駐。
4. **超大物件回收要判斷**：某次 log 超長把 buffer 撐到很大，`Put` 回去會讓 pool 長期佔大記憶體。可判斷 `cap` 太大就不 Put（丟掉讓 GC）。
5. **沒設 `New`**：空池 `Get` 回 `nil` → 要自己判斷。設了 `New` 就總有東西拿。

---

## 執行 / 驗證

```bash
cd interview/singleton-pool
go run .                          # 模擬高併發寫 log
go run -race .                    # 驗無競態
go test -bench . -benchmem -run '^$'  # 比較「有 pool vs 無 pool」的 allocs/op
```

- **sync.Once 驗證**：用 `atomic` counter 記「初始化實際被執行幾次」，開 1 萬 goroutine 併發呼叫 → counter 應為 **1**。
- **sync.Pool 驗證**：benchmark `-benchmem` 對比「有池 vs 無池」的 `allocs/op` 與 `B/op` → 有池版分配次數大降。也可用 `testing.AllocsPerRun`。

---

## 面試收束

1. **初始化只一次**：`sync.Once` 的 `Do` 保證併發下只執行一次，之後走 atomic 快路徑；勝過「if nil」（race）和「Mutex」（熱路徑一直鎖）。
2. **減 GC**：`sync.Pool` 重用 buffer，把百萬短命物件變成重複利用，分配次數 → GC 壓力雙降。
3. **Pool 三鐵律**：拿到先 **Reset**、**Put 後別再碰**、**別當永久快取/放連線**。
