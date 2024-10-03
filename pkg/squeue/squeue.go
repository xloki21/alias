package squeue

type Queue struct {
	queue chan any
}

func New() *Queue {
	return &Queue{queue: make(chan any)}
}

func (q *Queue) Produce(data any) {
	q.queue <- data
}

func (q *Queue) Consume() chan any {
	return q.queue
}
