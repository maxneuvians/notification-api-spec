package services

type InvalidRequestError struct {
	Message    string
	StatusCode int
}

func (e InvalidRequestError) Error() string {
	return e.Message
}

func (e InvalidRequestError) Body() map[string]string {
	return map[string]string{
		"result":  "error",
		"message": e.Message,
	}
}
