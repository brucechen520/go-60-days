package main

import (
	"context"
	"testing"
	"time"
)

// svc 產一個 ctx-aware 的假服務（延遲 d 後回 data；ctx 取消就提早退）。
func svc(name string, tier Tier, d time.Duration, data any) ServiceSpec {
	return ServiceSpec{Name: name, Tier: tier, Call: func(ctx context.Context) (any, error) {
		select {
		case <-time.After(d):
			return data, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
}

// 可選服務超時 → 回 partial + Failed 標記，不回 error。
func TestV2_OptionalTimeout(t *testing.T) {
	services := []ServiceSpec{
		svc("product", Critical, 100*time.Millisecond, "iPhone 16"),
		svc("stock", Optional, 100*time.Millisecond, 42),
		svc("reviews", Optional, 5*time.Second, "..."), // 超時（budget 500ms）
	}
	res, err := aggregateV2(context.Background(), 500*time.Millisecond, services)

	if err != nil {
		t.Fatalf("可選服務超時不該回 error，卻得到: %v", err)
	}
	if res.Data["product"] != "iPhone 16" {
		t.Errorf("product 應有值，得到 %v", res.Data["product"])
	}
	if res.Data["stock"] != 42 {
		t.Errorf("stock 應有值，得到 %v", res.Data["stock"])
	}
	if len(res.Failed) != 1 || res.Failed[0] != "reviews" {
		t.Errorf("reviews 應在 Failed，得到 %v", res.Failed)
	}
}

// 必要服務超時 → 回 error（整個請求失敗）。
func TestV2_CriticalTimeout(t *testing.T) {
	services := []ServiceSpec{
		svc("product", Critical, 5*time.Second, "iPhone 16"), // critical 超時
		svc("stock", Optional, 100*time.Millisecond, 42),
	}
	_, err := aggregateV2(context.Background(), 500*time.Millisecond, services)

	if err == nil {
		t.Fatal("必要服務超時應回 error，卻回 nil")
	}
}

// 全部成功 → 完整資料、無 Failed、無 error。
func TestV2_AllOK(t *testing.T) {
	services := []ServiceSpec{
		svc("product", Critical, 50*time.Millisecond, "iPhone 16"),
		svc("stock", Optional, 50*time.Millisecond, 42),
		svc("reviews", Optional, 50*time.Millisecond, []string{"讚"}),
	}
	res, err := aggregateV2(context.Background(), 500*time.Millisecond, services)

	if err != nil {
		t.Fatalf("全部成功不該回 error: %v", err)
	}
	if len(res.Data) != 3 {
		t.Errorf("應拿到 3 筆，得到 %d", len(res.Data))
	}
	if len(res.Failed) != 0 {
		t.Errorf("不該有 Failed，得到 %v", res.Failed)
	}
}
