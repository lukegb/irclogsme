package logger

import (
	"labix.org/v2/mgo"
)

type Database struct {
	connection mgo.Session
}
