package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup

	wg.Add(1) // 告訴程式我有一個任務要跑

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic in main:", r)
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
		}
	}()

	go case0(&wg) // 啟動一個 goroutine 執行 case

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
