package main

import (
	"fmt"
	"strings"

	"golang.org/x/tour/wc"
)

// K 必須滿足 comparable 約束，因為 map 的 Key 需要比對是否重複
// V 可以是任何型別 (any)
func printMap[K comparable, V any](m map[K]V) {
	for k, v := range m {
		fmt.Printf("map[%v]: %v\n", k, v)
	}
}

func WordCount(s string) map[string]int {
	res := make(map[string]int)

	words := strings.Fields(s)

	for _, word := range words {
		res[word]++
	}

	return res
}

func main() {
	wc.Test(WordCount)

	m := make(map[string]int)

	m["Answer"] = 42
	printMap(m)

	m["Answer"] = 48
	v, ok := m["Answer"]
	fmt.Println("The Answer value:", v, "OK?", ok)

	delete(m, "Answer")

	v, ok = m["Answer"]
	fmt.Println("The Answer value:", v, "OK?", ok) // value is zero -> 1. zero value. 2. really zero and ok is true

	var p map[string]int // 僅宣告，此時 m 是 nil

	fmt.Println(p)
	fmt.Println(p["apple"])
	p["apple"] = 10 // 💥 Panic: assignment to entry in nil map -> need to initialize with make

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recover the 💥 Panic: assignment to entry in nil map", r)
		}
	}()
}
