package main

import "fmt"

func main() {
	var s string
	// s = "People"
	s = "Go語言"

	/**
	* 索引：0, 字元：G (Unicode: U+0047)
	* 索引：1, 字元：o (Unicode: U+006F)
	* 索引：2, 字元：語 (Unicode: U+8A9E)
	* 索引：5, 字元：言 (Unicode: U+8A00)
	 */
	for i, r := range s {
		fmt.Printf("索引：%d, 字元：%c (Unicode: %U)\n", i, r, r)
	}

	for i := 0; i < len(s); i++ {
		fmt.Printf("%x ", s[i]) // 這印出的是原始的位元組 (Byte)
	}

	count := 0
	for _, r := range s {
		fmt.Printf("%d -> %c\n", count, r)
		count++
	}

	fmt.Println()
	// 將字串轉為 rune 切片
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		fmt.Printf("Index %d -> %c\n", i, runes[i])
	}
}
