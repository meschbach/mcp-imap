package mcpimap

import "fmt"

type operationalError struct {
	operation string
	problem   error
}

func (o *operationalError) Unwrap() error {
	return o.problem
}

func (o *operationalError) Error() string {
	return fmt.Sprintf("%s error: %s", o.operation, o.problem)
}
