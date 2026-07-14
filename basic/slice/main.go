package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// ref https://go.dev/tour/moretypes/18
func Pic(dx, dy int) [][]uint8 {
	picture := make([][]uint8, dy)

	for y := 0; y < dy; y++ {
		picture[y] = make([]uint8, dx)

		for x := 0; x < dx; x++ {
			picture[y][x] = uint8(x ^ y) // try (x+y)/2, x*y, and x^y.
		}
	}

	return picture
}

func saveImage(filename string, data [][]uint8) error {
	dy := len(data)
	if dy == 0 {
		return fmt.Errorf("empty data")
	}
	dx := len(data[0])

	m := image.NewNRGBA(image.Rect(0, 0, dx, dy))
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			v := data[y][x]
			m.Set(x, y, color.RGBA{v, v, 255, 255})
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close() // 使用 defer 確保檔案一定會關閉

	return png.Encode(f, m)
}

func printSlice[T any](s []T) {
	fmt.Printf("len=%d cap=%d %v\n", len(s), cap(s), s)
}

func main() {
	i := [1]int{1}
	fmt.Println(i)

	s := []int{2, 3, 5, 7, 11, 13}
	printSlice(s)

	// Slice the slice to give it zero length.
	s = s[:0]
	printSlice(s)

	// Extend its length.
	s = s[:4]
	printSlice(s)

	// Drop its first two values.
	s = s[2:]
	printSlice(s)

	pow := make([]int, 5, 10)
	printSlice(pow)

	// range 只有一個參數的話，會回傳 index
	for i := range pow {
		fmt.Printf("%d\n", i)
		pow[i] = 1 << uint(i) // == 2**i
	}
	// range 兩個參數的話，第一個是 index，第二個是 value
	for _, value := range pow {
		fmt.Printf("%d\n", value)
	}
	for value, _ := range pow {
		fmt.Printf("%d\n", value)
	}
	printSlice(pow)

	pow = append(pow, 7, 8, 9)
	printSlice(pow)

	people := []string{"Bob", "Lily"}
	printSlice(people)

	// 💥 PANIC: Runtime Error
	p := make([]int, 3, 5) // len=3, 索引只有 0, 1, 2
	fmt.Println(p[3])      // 💥 Panic: runtime error: index out of range [3] with length 3
	// fmt.Println(p[:8]) // 💥 panic: runtime error: slice bounds out of range [:8] with capacity 5

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recover the index out of range: ", r)
		}
	}()

	// saveImage("slice/output2.png", Pic(256, 256))
}
