package main

import "fmt"

func main() {
	defer A()
	defer C()
	fmt.Println(test())
}

func A() {
	fmt.Println("Function A is called")
}

func B() {
	fmt.Println("Function B is called")
}

func C() {
	fmt.Println("Function C is called")
	defer B()
}

// 具名回傳值的 defer 會在函式回傳前執行，
// 因此在這個例子中，當 test 函式執行到 return 100 時，i 的值已經被設定為 100，
// 但在 defer 中 i++ 的操作會在 return 之前執行，所以最終返回的值是 101。
func test() (i int) {
	defer func() {
		i++
	}()
	return 100
}
