package logger

import (
	irc "github.com/fluffle/goirc/client"
	"log"
)

func main() {
	ALL_CLIENTS := []irc.Conn{}
	log.Println(ALL_CLIENTS)
	log.Println("Hi, I'm Daisy!")
}
