package core

import "sync"

// PasswordQueue manages the list of passwords to try
type PasswordQueue struct {
	passwords []string
	index     int
	mu        sync.Mutex
}

// NewPasswordQueue creates a new password queue
func NewPasswordQueue(passwords []string) *PasswordQueue {
	return &PasswordQueue{
		passwords: passwords,
		index:     0,
	}
}

// Next returns the next password in the queue, or empty string if done
func (pq *PasswordQueue) Next() string {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.index >= len(pq.passwords) {
		return ""
	}

	password := pq.passwords[pq.index]
	pq.index++
	return password
}

// Unget returns the last password back to the queue (rewind by 1)
// Used when a password attempt fails due to connection error and needs to be retried
func (pq *PasswordQueue) Unget() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.index > 0 {
		pq.index--
	}
}

// Reset resets the queue to the beginning
func (pq *PasswordQueue) Reset() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.index = 0
}

// Progress returns the current progress (0.0 to 1.0)
func (pq *PasswordQueue) Progress() float64 {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.passwords) == 0 {
		return 0.0
	}
	return float64(pq.index) / float64(len(pq.passwords))
}

// Total returns the total number of passwords
func (pq *PasswordQueue) Total() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.passwords)
}

// Remaining returns the number of passwords remaining
func (pq *PasswordQueue) Remaining() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.passwords) - pq.index
}
