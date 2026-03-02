package main

// ref: https://go.cyub.vip/feature/panic-recover/

// ref https://go.dev/blog/defer-panic-and-recover
// Panic is a built-in function that stops the ordinary flow of control and begins panicking.
// When the function F calls panic, execution of F stops, any deferred functions in F are executed normally,
// and then F returns to its caller. To the caller, F then behaves like a call to panic.
// The process continues up the stack until all functions in the current goroutine have returned, at which point the program crashes.
// Panics can be initiated by invoking panic directly.
// They can also be caused by runtime errors, such as out-of-bounds array accesses.

// Recover is a built-in function that regains control of a panicking goroutine.
// Recover is only useful inside deferred functions.
// During normal execution, a call to recover will return nil and have no other effect.
// If the current goroutine is panicking, a call to recover will capture the value given to panic and resume normal execution.

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup

	wg.Add(1) // 告訴程式我有一個任務要跑

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
		}
	}()

	go case1(&wg) // 啟動一個 goroutine 執行 case

	f() // ref https://go.dev/blog/defer-panic-and-recover
	fmt.Println("Returned normally from f.")

	wg.Wait() // 等待所有任務完成
}

/**
* 在這個範例中，case0 函式會引發 panic，但由於我們在 main 函式中使用了 recover，程式依然會崩潰
*，因為他是在 go routine中引起的 panic，並且 recover 只能捕捉到同一個 goroutine 中的 panic。
 */
func case0(wg *sync.WaitGroup) {
	defer wg.Done()
	panic("Something went wrong!")
}

/**
* 在這個範例中，case1 函式會引發 panic，但由於我們在 case1 函式內部使用了 recover，所以程式不會崩潰，而是會捕捉到 panic 並輸出相關訊息。
 */
func case1(wg *sync.WaitGroup) {
	defer wg.Done() // 任務完成後告訴 WaitGroup
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic in goroutine:", r)
		}
	}()

	panic("Something went wrong!")
}

func f() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()
	fmt.Println("Calling g.")
	g(0)
	fmt.Println("Returned normally from g.")
}

func g(i int) {
	if i > 3 {
		fmt.Println("Panicking!")
		panic(fmt.Sprintf("%v", i))
	}
	defer fmt.Println("Defer in g", i)
	fmt.Println("Printing in g", i)
	g(i + 1)
}
