package main

import "fmt"

type WorkerOption func(*Worker)

type Worker struct {
	Name string
	Age  int
	Job  string
}

func New(options ...WorkerOption) *Worker {
	worker := new(Worker)

	for _, option := range options {
		option(worker)

	}
	return worker
}

func WithName(name string) WorkerOption {
	return func(worker *Worker) {
		worker.Name = name
	}
}

func WithAge(age int) WorkerOption {
	return func(worker *Worker) {
		worker.Age = age
	}
}

func WithJob(job string) WorkerOption {
	return func(worker *Worker) {
		worker.Job = job
	}
}

func (w *Worker) Work() {
	fmt.Printf("%s is working as a %s.\n", w.Name, w.Job)
}

func main() {
	worker := New(WithName("Tux"), WithAge(34), WithJob("Software Engineer"))

	fmt.Println(`Worker Details:`, worker.Name, worker.Age, worker.Job)
	worker.Work()
}
