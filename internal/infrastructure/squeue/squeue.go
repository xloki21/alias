package squeue

type EventQueue struct {
	queue chan any
}

func New() *EventQueue {
	return &EventQueue{queue: make(chan any)}
}

func (q *EventQueue) Produce(data any) {
	q.queue <- data
}

func (q *EventQueue) Consume() chan any {
	return q.queue
}
