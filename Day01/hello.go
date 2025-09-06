package main

import "fmt"

func main() {
	// 輸出固定訊息
	fmt.Println("Hello, World!")

	// 輸出變數內容
	user := "Tux"
	fmt.Printf("Hello, %s! \n", user)

	// 字串合併輸出
	first := "Eason"
	second := "Tux"
	fmt.Println("Hello, " + first + " " + second + "!")
}
