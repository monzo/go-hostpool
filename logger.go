package hostpool

import "log"

type Logger interface {
	Printf(msg string, params ...interface{})
	Println(msg string)
	Fatalf(msg string, params ...interface{})
}

type DefaultLogger struct{}

func (l DefaultLogger) Printf(msg string, params ...interface{}) {
	log.Printf(msg, params...)
}

func (l DefaultLogger) Println(msg string) {
	log.Println(msg)
}

func (l DefaultLogger) Fatalf(msg string, params ...interface{}) {
	log.Fatalf(msg, params...)
}
