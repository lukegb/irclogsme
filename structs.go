package irclogsme

import (
	"fmt"
	"labix.org/v2/mgo/bson"
	"time"
)

const (
	LMT_PRIVMSG = iota
	LMT_NOTICE
	LMT_JOIN
	LMT_PART
	LMT_TOPIC
	LMT_KICK
	LMT_QUIT
)

const (
	CMT_START_LOGGING = iota
	CMT_STOP_LOGGING
	CMT_DISCONNECT
	CMT_CONNECT
	CMT_TELL
)

type LogMessageType uint
type CommandMessageType uint

func (l LogMessageType) String() string {
	switch l {
	case LMT_PRIVMSG:
		return "PRIVMSG"
	case LMT_NOTICE:
		return "NOTICE"
	case LMT_JOIN:
		return "JOIN"
	case LMT_PART:
		return "PART"
	case LMT_TOPIC:
		return "TOPIC"
	case LMT_KICK:
		return "KICK"
	case LMT_QUIT:
		return "QUIT"
	}
	return fmt.Sprintf("[unknown %d]", l)
}

func (c CommandMessageType) String() string {
	switch c {
	case CMT_START_LOGGING:
		return "START_LOGGING %s/%s"
	case CMT_STOP_LOGGING:
		return "STOP_LOGGING"
	case CMT_DISCONNECT:
		return "DISCONNECT"
	case CMT_CONNECT:
		return "CONNECT"
	case CMT_TELL:
		return "TELL"
	}
	return fmt.Sprintf("[unknown %d]", c)
}

type LogMessage struct {
	Id bson.ObjectId `bson:"_id,omitempty"`

	NetworkId bson.ObjectId
	Channel   string

	Time      time.Time
	SplitDate string

	Nick  string
	Ident string
	Host  string

	Type LogMessageType

	Target  interface{}
	Payload interface{}
}

type CommandMessage struct {
	Id bson.ObjectId `bson:"_id,omitempty"`

	NetworkId bson.ObjectId
	Type      CommandMessageType
	Channel   string
	Target    string
	Message   string
	Complete  bool
}

type ChannelConfig struct {
}

type NetworkConfig struct {
	Id bson.ObjectId `bson:"_id,omitempty"`

	Name string

	Channels map[string]ChannelConfig

	Nick    string
	Enabled bool
	User    string

	IrcServers []string

	AuthCommands []string
}

type Config struct {
	Networks []NetworkConfig
}
