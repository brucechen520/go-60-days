package main

import "fmt"

func main() {
	// case 1 - 函式作為變數
	fn := add                           // 將 add 函式賦值給 fn 變數，fn 現在是一個函式類型的變數，可以像函式一樣被調用
	fmt.Println("add(1, 2):", fn(1, 2)) // 3

	// case 2 - 函式回傳一個函式
	pow2 := pow(2)                   // 調用 pow 函式，傳入 2，返回一個函式，這個函式可以接受一個參數 b，並且在內部使用 a 的值來計算 2 的 b 次方
	fmt.Println("pow 2^3:", pow2(3)) // 8
	fmt.Println("pow 2^4:", pow2(4)) // 16

	// case 3 - 函式作為參數
	data := []int{1, 2, 3, 4, 5}
	fmt.Println("filter even numbers:", filter(data, func(n int) bool {
		return n%2 == 0
	}))

	fmt.Println("filter odd numbers:", filter(data, func(n int) bool {
		return n%2 == 1
	}))

	// case 4 -- 閉包
	generate := generateInteger()
	fmt.Println(generate()) // 0
	fmt.Println(generate()) // 1
	fmt.Println(generate()) // 2

	// case 5 -- 泛型實踐
	strData := []string{"apple", "banana", "cherry", "date"}
	fmt.Println("filter strings with length > 5:", filterGenerics(strData, func(s string) bool {
		return len(s) > 5
	}))

	people := []struct {
		Name string
		Age  int
	}{
		{
			Name: "Alice",
			Age:  30,
		},
		{
			Name: "Bob",
			Age:  25,
		},
	}
	fmt.Println("filter people with age > 28:", filterGenerics(people, func(p struct {
		Name string
		Age  int
	}) bool {
		return p.Age > 28
	}))
}

func add(a, b int) int {
	return a + b
}

// @Summary 計算次方
// @Description 接收一個基數 a，回傳一個函式，該函式接受一個指數 b，並計算 a 的 b 次方
// @Param a query int true "基數"
// @Return func(b int) int "計算 a 的 b 次方的函式"
func pow(a int) func(b int) int {
	return func(b int) int {
		result := 1
		for i := 0; i < b; i++ {
			result *= a
		}

		return result
	}
}

// @Summary 過濾切片
// @Description 接收一個 int 切片與判斷函式，回傳符合條件的元素
// @Param a query []int true "原始數據"
// @Return []int "符合條件的元素"
func filter(a []int, fn func(int) bool) (result []int) {
	for _, v := range a {
		if fn(v) {
			result = append(result, v)
		}
	}

	return result
}

// @Summary 生成整數 (Closure)
// @Description 回傳一個函式，每次調用該函式會返回一個遞增的整數
// @Return func() int "生成整數的函式"
func generateInteger() func() int {
	ch := make(chan int)
	count := 0

	go func() {
		for {
			ch <- count
			count++
		}
	}()

	return func() int {
		return <-ch
	}
}

// @Summary 過濾切片 (以泛型實踐)
// @Description 接收一個 T 類型的切片與判斷函式，回傳符合條件的元素
// @Param a query []T true "原始數據"
// @Return []T "符合條件的元素"
func filterGenerics[T any](data []T, fn func(T) bool) (result []T) {
	for _, v := range data {
		if fn(v) {
			result = append(result, v)
		}
	}

	return result
}
