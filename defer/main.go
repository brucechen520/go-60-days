package main

import "fmt"

func main() {
	// defer A()
	// defer C()
	// fmt.Println(test())

	a()
	b()
	// fmt.Println()
	// fmt.Println("c():", c())
}

// defer 3 rules: https://go.dev/blog/defer-panic-and-recover
// First rule, A deferred function’s arguments are evaluated when the defer statement is evaluated.
func a() {
	i := 0
	defer fmt.Println(i)
	i++
	return
}

// Second rule, Deferred function calls are executed in Last In First Out order after the surrounding function returns.
// This function prints “3210”:
func b() {
	for i := 0; i < 4; i++ {
		defer fmt.Print(i)
	}
}

// Third rule, Deferred functions may read and assign to the returning function’s named return values.
// In this example, a deferred function increments the return value i after the surrounding function returns.
// Thus, this function returns 2:
func c() (i int) {
	defer func() { i++ }()
	return 1
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
