package main

import "fmt"

func main() {
	var i interface{} = "hello"

	s := i.(string)
	fmt.Println(s)

	s, ok := i.(string)
	fmt.Println(s, ok)

	sli, ok := i.([]string)
	fmt.Println(sli, ok)

	arr, ok := i.([3]int)
	fmt.Println(arr, ok)

	m, ok := i.(map[string]int)
	fmt.Println(m, ok)

	f, ok := i.(float64)
	fmt.Println(f, ok)

	f = i.(float64) // panic
	fmt.Println(f)

}
