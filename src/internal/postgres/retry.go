package postgres

type retryableFunc[T any] func() (T, error)

func runWithRetry[T any](fn retryableFunc[T]) (T, error) {
	var err error
	var t T

	t, err = fn()
	if err != nil && canRetry(err) {
		t, err = fn()
	}

	return t, err
}

func canRetry(err error) bool {

	// TODO: transient error handling (deadlock, timeout ant etc.) with retry
	// sql.DB has some retry logic internally, but not sure what exactly
	// if it can handle deadlocks the we can get rid of this...

	return false
}
