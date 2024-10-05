package squeue

import (
	"context"
	"errors"
	"fmt"
)

type MessageQueue struct {
	queue chan any
}

func New() *MessageQueue {
	return &MessageQueue{queue: make(chan any)}
}

func (q *MessageQueue) WriteMessage(ctx context.Context, msg any) error {
	q.queue <- msg
	return nil
}

func (q *MessageQueue) ReadMessage(ctx context.Context) (any, error) {
	message, ok := <-q.queue
	if !ok {
		return nil, errors.New("channel closed")
	}
	return message, nil
}

func (q *MessageQueue) CommitMessages(ctx context.Context, messages ...any) error {
	return nil
}

func (q *MessageQueue) Close() {
	close(q.queue)
}

func (q *MessageQueue) Consume(ctx context.Context, consumer func(ctx context.Context, message any) error) error {
	msg, err := q.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}
	if err := consumer(ctx, msg); err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	if err := q.CommitMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to commit message: %w", err)
	}
	return nil
}
