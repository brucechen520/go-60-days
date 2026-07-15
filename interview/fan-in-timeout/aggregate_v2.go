package main

import (
	"context"
	"fmt"
	"time"
)

// Tier 服務分級。
type Tier int

const (
	Critical Tier = iota // 必要：失敗或超時 → 整個請求回 error
	Optional             // 可選：失敗或超時 → 記進 Failed，回 partial
)

// ServiceSpec 一個服務的規格：名稱、分級、呼叫函式。
type ServiceSpec struct {
	Name string
	Tier Tier
	Call func(context.Context) (any, error)
}

// AggregateResult：成功拿到的資料 + 失敗/超時的可選服務清單。
// （critical 失敗會直接回 error，不會進到這裡。）
type AggregateResult struct {
	Data   map[string]any
	Failed []string
}

// aggregateV2 是 v1 的進階版，多了「服務分級」：
//   - critical 服務失敗或超時 → 回 error（頁面沒它沒意義，例如商品資訊）
//   - optional 服務失敗或超時 → 記進 Failed、回 partial（例如評價、庫存）
//
// 這解決 v1 的兩個缺口：
//  1. v1 三個服務一視同仁——product 掛了也只是 map 少一個 key，呼叫端不知道是「必要服務掛了」
//  2. v1 的 map 少 key 是歧義的——「沒資料」還是「超時」分不出來；v2 用 Failed 明確標記
func aggregateV2(ctx context.Context, budget time.Duration, services []ServiceSpec) (AggregateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	tierOf := make(map[string]Tier, len(services))
	results := make(chan Result, len(services)) // buffered = N，防洩漏（同 v1）

	for _, s := range services {
		tierOf[s.Name] = s.Tier
		go func(s ServiceSpec) {
			data, err := s.Call(ctx) // 傳 ctx → 超時時底層被取消
			results <- Result{Name: s.Name, Data: data, Err: err}
		}(s)
	}

	res := AggregateResult{Data: make(map[string]any)}
	reported := make(map[string]bool, len(services)) // 已回報的服務

loop:
	for i := 0; i < len(services); i++ {
		select {
		case r := <-results:
			reported[r.Name] = true
			if r.Err != nil {
				if tierOf[r.Name] == Critical {
					// 必要服務失敗 → 整個請求失敗
					return res, fmt.Errorf("critical service %q failed: %w", r.Name, r.Err)
				}
				res.Failed = append(res.Failed, r.Name) // 可選服務失敗 → 記下、繼續
			} else {
				res.Data[r.Name] = r.Data
			}
		case <-ctx.Done():
			break loop // 超時 → 跳出，未回報的服務在下面統一判斷
		}
	}

	// 超時後：還沒回報的服務都算「超時」。critical 超時 → error；optional → Failed。
	// （邊界情境：剛好在超時瞬間才送達、還沒被讀到的結果，這裡會被當成超時處理——
	//   對「2 秒硬截止」的語意是可接受的取捨；要更精準可在 break 後非阻塞 drain 一次 channel。）
	for _, s := range services {
		if reported[s.Name] {
			continue
		}
		if s.Tier == Critical {
			return res, fmt.Errorf("critical service %q timed out: %w", s.Name, ctx.Err())
		}
		res.Failed = append(res.Failed, s.Name)
	}
	return res, nil
}
