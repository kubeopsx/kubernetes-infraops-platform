package testutil

import "time"

// A MockContext provides a simple stub implementation of a Context
type MockContext struct {
	Error  error
	DoneCh chan struct{}
}

// Deadline always will return not set
func (c *MockContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done returns a read channel for listening to the Done event
func (c *MockContext) Done() <-chan struct{} {
	return c.DoneCh
}

// Err returns the error, is nil if not set.
func (c *MockContext) Err() error {
	return c.Error
}

// Value ignores the Value and always returns nil
func (c *MockContext) Value(key interface{}) interface{} {
	return nil
}
