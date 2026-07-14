package main

import (
	"fmt"
	"math"
	"time"
)

type MyError struct {
	When time.Time
	What string
}

func (e *MyError) Error() string {
	return fmt.Sprintf("at %v, %s",
		e.When, e.What)
}

// MyError defines a custom error structure for tracking business logic failures.
//
// Why use a Pointer Receiver (*MyError) for the Error() method?
// 1. Interface Consistency:
//   - The 'error' interface is often expected to be nil on success.
//   - Returning a pointer (&MyError) makes the nil-check semantically equivalent
//     to checking for an empty memory address, which is the standard Go idiom.
//
// 2. Performance & Efficiency:
//   - Avoids copying the entire struct (which contains time.Time and string)
//     on every method call or error propagation up the stack.
//   - Passing an 8-byte pointer is more efficient than copying the struct's data.
//
// 3. Error Type Assertion & Wrapping:
//   - Enables the use of 'errors.As(err, &target)', which requires the target
//     to be a pointer to a pointer. Most standard library errors (like *os.PathError)
//     follow this pointer-based convention.
//
// 4. State Modification (Future-proofing):
//   - If future methods need to modify error state (e.g., updating a retry count),
//     a pointer receiver is required to ensure changes persist across the call stack.
func run() error {
	return &MyError{
		time.Now(),
		"it didn't work",
	}
}

type ErrNegativeSqrt float64

func (e ErrNegativeSqrt) Error() string {
	return fmt.Sprintf("cannot Sqrt negative number: %v", float64(e))
}

func Sqrt(x float64) (float64, error) {
	if x < 0 {
		return 0, ErrNegativeSqrt(x)
	}

	z := float64(1)

	fmt.Printf("開始計算 %g 的平方根：\n", x)

	// 先執行 10 次循環觀察變化
	for i := 1; i <= 10; i++ {
		// 記錄舊的 z 用於比較
		oldZ := z

		// 牛頓法迭代公式
		z -= (z*z - x) / (2 * z)

		fmt.Printf("第 %d 次迭代: z = %v, oldZ = %v\n", i, z, oldZ)

		// 進階練習：當變化量極小時停止（例如 1e-10）
		if math.Abs(z-oldZ) < 1e-12 {
			fmt.Printf("--- 在第 %d 次迭代達到穩定 ---\n", i)
			break
		}
	}
	return z, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println(Sqrt(2))
	fmt.Println(Sqrt(-2))
}
