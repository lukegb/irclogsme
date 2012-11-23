package logger

import (
	"errors"
	"fmt"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"regexp"
	"time"
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

type LogMessage struct {
	NetworkId bson.ObjectId
	Channel   string

	Time time.Time

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

type Database interface {
	Connect(connString string) error
	GetConfig() (Config, error)
	FetchPendingCommands() ([]CommandMessage, error)
	CommandComplete(CommandMessage) error

	LogMessage(message LogMessage) error
}

var correctConnStringRegexp = regexp.MustCompile(`^([a-z]+)://.*`)
var errUrlBadFormat = errors.New(`url must be in format databaseprovider://databasestring`)

func DbConnect(dbConnString string) (Database, error) {
	LogDebug("told to connect to database with str %s", dbConnString)
	submatches := correctConnStringRegexp.FindStringSubmatch(dbConnString)

	if submatches == nil {
		return nil, errUrlBadFormat
	}

	dbp := submatches[1]
	var db Database
	if dbp == "mock" {
		db = new(MockDatabase)
	} else if dbp == "mongodb" {
		db = new(MongoDatabase)
	} else {
		return nil, errors.New(`no such database provider ` + dbp)
	}

	if err := db.Connect(dbConnString); err != nil {
		return nil, err
	}

	return db, nil
}

type MongoDatabase struct {
	connection *mgo.Session
}

func (m *MongoDatabase) Connect(connString string) error {
	var err error

	LogDebug("mongodb: connecting")
	m.connection, err = mgo.Dial(connString)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoDatabase) validateSelf() error {
	if m.connection == nil {
		return errors.New(`mongodb: connection not connected`)
	}
	return nil
}

func (m *MongoDatabase) GetConfig() (Config, error) {
	var c Config

	if err := m.validateSelf(); err != nil {
		return c, err
	}

	LogDebug("mongodb: fetching config")
	c = Config{}
	q := m.connection.DB("").C("config").Find(struct{}{})
	if count, err := q.Count(); count != 1 || err != nil {
		if err != nil {
			return c, err
		} else {
			return c, errors.New(`mongodb: no config in database`)
		}
	}
	if err := q.One(&c); err != nil {
		return c, err
	}

	LogDebug("mongodb: fetching networks")
	q = m.connection.DB("").C("networks").Find(struct{}{})
	if count, err := q.Count(); count == 0 || err != nil {
		if err != nil {
			return c, err
		} else {
			return c, errors.New(`mongodb: no networks in database`)
		}
	}

	if err := q.Iter().All(&c.Networks); err != nil {
		return c, err
	}

	for _, net := range c.Networks {
		LogDebug(" - loaded network: %s - servers are %s (connecting as %s!%s)", net.Name, net.IrcServers, net.Nick, net.User)
	}

	return c, nil
}

func (m *MongoDatabase) LogMessage(message LogMessage) error {
	if err := m.validateSelf(); err != nil {
		return err
	}

	LogDebug("mongodb: logging message - %s", message)
	if err := m.connection.DB("").C("logs").Insert(message); err != nil {
		return err
	}

	return nil
}

func (m *MongoDatabase) FetchPendingCommands() ([]CommandMessage, error) {
	if err := m.validateSelf(); err != nil {
		return nil, err
	}

	LogDebug("mongodb: fetching pending commands")
	q := m.connection.DB("").C("command_queue").Find(bson.M{"complete": false})

	commandMessageArray := make([]CommandMessage, 0)
	if err := q.Iter().All(&commandMessageArray); err != nil {
		return nil, err
	}

	return commandMessageArray, nil
}

func (m *MongoDatabase) CommandComplete(cmdMsg CommandMessage) error {
	if err := m.validateSelf(); err != nil {
		return err
	}

	LogDebug("mongodb: marking command as complete: %x", cmdMsg)
	if err := m.connection.DB("").C("command_queue").Update(bson.M{"_id": cmdMsg.Id}, bson.M{"$set": bson.M{"complete": true}}); err != nil {
		return err
	}

	return nil
}

type MockDatabase struct {
}

func (m *MockDatabase) Connect(connString string) error {
	LogDebug("mockdb: connecting")
	return nil
}

func (m *MockDatabase) GetConfig() (Config, error) {
	LogDebug("mockdb: fetching config")
	return Config{}, nil
}

func (m *MockDatabase) LogMessage(message LogMessage) error {
	return nil
}

func (m *MockDatabase) FetchPendingCommands() ([]CommandMessage, error) {
	return make([]CommandMessage, 0), nil
}

func (m *MockDatabase) CommandComplete(cmdMsg CommandMessage) error {
	return nil
}
