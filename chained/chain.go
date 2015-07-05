package chained

import "bytes"

type ChainedError struct {
	Message string
	Cause   error
}

func RootCause(err error) error {
	chained, ok := err.(ChainedError)
	if ok {
		return RootCause(chained.Cause)
	}
	return err
}

func (e ChainedError) Error() string {
	message := e.Message
	if e.Cause != nil {
		message += "\n" + e.Cause.Error()
	}
	return message
}

func joinStrings(separator string, message ...string) string {
	var buffer bytes.Buffer

	for i, m := range message {
		if i != 0 {
			buffer.WriteString(separator)
		}
		buffer.WriteString(m)
	}
	return buffer.String()
}

func Error(err error, message ...string) error {
	e := ChainedError{}
	e.Message = joinStrings(" ", message...)
	e.Cause = err
	return e
}
