// MIT License
//
// Copyright (c) 2022 Tommy TIAN
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package threadpoolexec

// task is a job to be executed by a worker.
type task[A, R any] struct {
	handler func(A) R
	args    A
	result  chan R
	stop    bool
}

type worker[A, R any] struct {
	jobChan chan task[A, R]
}

func newWorker[A, R any](jobChan chan task[A, R]) *worker[A, R] {
	w := &worker[A, R]{
		jobChan: jobChan,
	}
	go w.run()
	return w
}

func (w *worker[A, R]) run() {
	for job := range w.jobChan {
		if job.stop {
			return
		}
		job.result <- job.handler(job.args)
	}
}