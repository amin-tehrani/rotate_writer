package rotate_writer

type RotateError struct {
	err error
}

func (re *RotateError) Error() string {
	return "failed to rotate file: " + re.err.Error()
}
