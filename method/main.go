package main

import (
	"fmt"
	"math"
)

// value reciever
type Vertex struct {
	X, Y float64
}

// ref: https://go.dev/tour/methods/3
// You can only declare a method with a receiver whose type is defined in the same package as the method.
// You cannot declare a method with a receiver whose type is defined in another package (which includes the built-in types such as int).
func (v Vertex) Abs() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

func Abs(v Vertex) float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

type MyFloat float64

func (f MyFloat) Abs() float64 {
	if f < 0 {
		return float64(-f)
	}
	return float64(f)
}

// pointer reciever
// Methods with pointer receivers can modify the value to which the receiver points (as Scale does here).
// Since methods often need to modify their receiver, pointer receivers are more common than value receivers.
// Try removing the * from the declaration of the Scale function on line 16 and observe how the program's behavior changes.
// With a value receiver, the Scale method operates on a copy of the original Vertex value.
// (This is the same behavior as for any other function argument.)
// The Scale method must have a pointer receiver to change the Vertex value declared in the main function.
func (v *Vertex) Scale(f float64) {
	v.X = v.X * f
	v.Y = v.Y * f
}

func ScaleFunc(v *Vertex, f float64) {
	v.X = v.X * f
	v.Y = v.Y * f
}

func main() {
	v := Vertex{3, 4}
	fmt.Println(v.Abs())
	fmt.Println(Abs(v)) // equivalent as v.Abs(method)
	v.Scale(2)
	ScaleFunc(&v, 10)
	// v.Scale(10)

	p := &Vertex{4, 3}
	p.Scale(3)
	ScaleFunc(p, 8)

	fmt.Println(v, p)

	fmt.Println(v.Abs())

	f := MyFloat(-math.Sqrt2)
	fmt.Println(f.Abs())
}
