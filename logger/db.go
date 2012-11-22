package logger

import (
	"errors"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"regexp"
	"time"
)

type LogMessageType uint

const (
	LMT_PRIVMSG = iota
	LMT_NOTICE
	LMT_JOIN
	LMT_PART
	LMT_TOPIC
)

type LogMessage struct {
	NetworkId bson.ObjectId
	Channel   string

	Time time.Time

	Nick  string
	Ident string
	Host  string

	Type LogMessageType

	Payload interface{}
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
