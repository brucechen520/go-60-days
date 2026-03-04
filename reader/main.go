package main

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/tour/reader"
)

type MyReader struct{}

// TODO: Add a Read([]byte) (int, error) method to MyReader.

// MyReader demonstrates core principles of Go's I/O design and system architecture:
//
// 1. Inversion of Control (IoC) & Memory Efficiency:
//   - Unlike traditional I/O that returns a new buffer, Go's io.Reader requires
//     the caller to provide the buffer: "You give me the bucket, I fill it."
//   - This allows for buffer reuse, significantly reducing GC (Garbage Collection)
//     overhead, which is critical for high-concurrency systems like "kuji-go".
//
// 2. Implicit Interface Satisfaction (Interface as a Contract):
//   - By implementing the Read signature, this struct implicitly satisfies io.Reader.
//   - This "duck typing" enables seamless integration with the Go standard library,
//     allowing MyReader to be used anywhere a Reader is expected (e.g., io.Copy).
//
// 3. Infinite Stream Simulation:
//   - By never returning io.EOF, this reader simulates an endless data source.
//   - This pattern is foundational for handling real-time data like log streams,
//     WebSocket long-polling, or gRPC streaming where the data flow is continuous.
//
// 4. Architectural Implications:
//   - Stream-Oriented Processing: Prevents OOM (Out of Memory) by processing
//     large datasets in fixed-size chunks instead of loading entire files.
//   - Backpressure Management: Ensures that read speed is synchronized with
//     flush/write speed. By using blocking I/O, we prevent data accumulation
//     in memory when the downstream (e.g., network) is slower than the source.
func (r MyReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 'A'
	}

	return len(b), nil
}

func main() {
	r := strings.NewReader("Hello, Reader!")

	b := make([]byte, 8)
	for {
		// Note: Why r.Read(b) instead of r.Read(&b)?
		// 1. Slice Semantics: In Go, a slice is a header containing a pointer to an underlying array.
		// 2. Efficiency: Passing the slice by value still allows the function to modify the same
		//    memory space because the internal data pointer is copied.
		// 3. Interface Compliance: io.Reader expects []byte (the header), not *[]byte (pointer to header).
		n, err := r.Read(b)
		fmt.Printf("n = %v err = %v b = %v\n", n, err, b)
		fmt.Printf("b[:n] = %q\n", b[:n])
		if err == io.EOF {
			break
		}
	}
	fmt.Println("Implement Infinite stream reader!")
	reader.Validate(MyReader{})
}
