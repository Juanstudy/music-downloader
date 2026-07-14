package model

// Queue is an in-memory list of tracks to download.
// Matches MVP constraint: no persistence, lost on close.
type Queue struct {
	Tracks []Media
	Index  int // current track being processed, -1 if idle
}

// NewQueue creates an empty queue.
func NewQueue() *Queue {
	return &Queue{
		Index: -1,
	}
}

// Add appends one or more tracks.
func (q *Queue) Add(tracks ...Media) {
	q.Tracks = append(q.Tracks, tracks...)
}

// Len returns the total number of tracks.
func (q *Queue) Len() int { return len(q.Tracks) }

// Completed returns the number of completed tracks.
func (q *Queue) Completed() int {
	n := 0
	for _, t := range q.Tracks {
		if t.Status == StatusCompleted {
			n++
		}
	}
	return n
}

// Failed returns the number of failed tracks.
func (q *Queue) Failed() int {
	n := 0
	for _, t := range q.Tracks {
		if t.Status == StatusFailed {
			n++
		}
	}
	return n
}

// Pending returns the number of tracks not yet processed.
func (q *Queue) Pending() int {
	n := 0
	for _, t := range q.Tracks {
		if t.Status == StatusPending {
			n++
		}
	}
	return n
}

// Current returns the currently downloading track, or nil.
func (q *Queue) Current() *Media {
	if q.Index < 0 || q.Index >= len(q.Tracks) {
		return nil
	}
	return &q.Tracks[q.Index]
}

// Next advances to the next pending track. Returns false if none left.
func (q *Queue) Next() bool {
	for i := q.Index + 1; i < len(q.Tracks); i++ {
		if q.Tracks[i].Status == StatusPending {
			q.Tracks[i].Status = StatusDownloading
			q.Index = i
			return true
		}
	}
	q.Index = -1
	return false
}

// MarkCurrentCompleted marks the current track as completed.
func (q *Queue) MarkCurrentCompleted() {
	if cur := q.Current(); cur != nil {
		cur.Status = StatusCompleted
	}
}

// MarkCurrentFailed marks the current track as failed with an error.
func (q *Queue) MarkCurrentFailed(errMsg string) {
	if cur := q.Current(); cur != nil {
		cur.Status = StatusFailed
		cur.ErrorMsg = errMsg
	}
}
