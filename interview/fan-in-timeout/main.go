package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

type Result struct {
	Name string
	Data interface{}
	Err  error
}

func main() {
	base := runtime.NumGoroutine()
	fmt.Printf("base: %d\n", base)
	start := time.Now()
	got := aggregate(context.Background())
	fmt.Printf("got: %v, elapsed time: %v\n", got, time.Since(start))
	fmt.Printf("base: %d\n", runtime.NumGoroutine())
}

func fetchProduct(ctx context.Context) (any, error) {
	select {
	case <-time.After(300 * time.Millisecond): // 正常完成
		return "iPhone 16", nil
	case <-ctx.Done(): // 被取消/超時 → 立刻退
		return nil, ctx.Err()
	}
}

func fetchStock(ctx context.Context) (any, error) {
	select {
	case <-time.After(500 * time.Millisecond): // 正常完成
		return "Appl", nil
	case <-ctx.Done(): // 被取消/超時 → 立刻退
		return nil, ctx.Err()
	}
}

func fetchReviews(ctx context.Context) (any, error) {
	select {
	case <-time.After(5 * time.Second): // 正常完成
		return "Too many files", nil
	case <-ctx.Done(): // 被取消/超時 → 立刻退
		return nil, ctx.Err()
	}
}

func aggregate(ctx context.Context) map[string]any {
	ctx1, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel() // 確保 ctx 一定被取消，通知所有背景 goroutine 收工

	services := []struct {
		name string
		call func(context.Context) (any, error)
	}{
		{"product", fetchProduct},
		{"reviews", fetchReviews}, // 可能卡 5 秒
		{"stock", fetchStock},
	}

	m := make(map[string]any, len(services))

	results := make(chan Result, len(services))

	for _, s := range services {
		go func(name string, fn func(context.Context) (any, error)) {
			val, err := fn(ctx1)
			results <- Result{
				Name: name,
				Data: val,
				Err:  err,
			}
		}(s.name, s.call)
	}

	for i := 0; i < len(services); i++ {
		select {
		case v := <-results:
			if v.Err == nil {
				m[v.Name] = v.Data
			}
		case <-ctx1.Done():
			return m
		}
	}

	return m
}
