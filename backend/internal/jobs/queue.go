package jobs

import (
	"context"

	"last-deploy/internal/store"
)

type Queue struct {
	ch chan string
}

func NewQueue(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 1
	}
	return &Queue{ch: make(chan string, buffer)}
}

func (q *Queue) Enqueue(jobID string) {
	q.ch <- jobID
}

func (q *Queue) C() <-chan string {
	return q.ch
}

func EnqueuePersisted(ctx context.Context, st *store.Store, q *Queue) error {
	jobs, err := st.ListJobsByStatus(ctx, store.JobStatusQueued)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		q.Enqueue(j.ID)
	}
	return nil
}
