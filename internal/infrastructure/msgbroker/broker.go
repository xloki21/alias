package msgbroker

type Queue interface {
	Produce(data any)
	Consume() chan any
}
