package logger

import (
	"errors"
	"github.com/lukegb/irclogsme"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"regexp"
)

type Database interface {
	Connect(connString string) error
	GetConfig() (irclogsme.Config, error)
	FetchPendingCommands() ([]irclogsme.CommandMessage, error)
	CommandComplete(irclogsme.CommandMessage) error

	LogMessage(message irclogsme.LogMessage) error
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

func (m *MongoDatabase) GetConfig() (irclogsme.Config, error) {
	var c irclogsme.Config

	if err := m.validateSelf(); err != nil {
		return c, err
	}

	LogDebug("mongodb: fetching config")
	c = irclogsme.Config{}
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

func (m *MongoDatabase) LogMessage(message irclogsme.LogMessage) error {
	if err := m.validateSelf(); err != nil {
		return err
	}

	message.SplitDate = message.Time.Format("2006-01-02")

	LogDebug("mongodb: logging message - %s", message)
	if err := m.connection.DB("").C("logs").Insert(message); err != nil {
		return err
	}

	return nil
}

func (m *MongoDatabase) FetchPendingCommands() ([]irclogsme.CommandMessage, error) {
	if err := m.validateSelf(); err != nil {
		return nil, err
	}

	LogDebug("mongodb: fetching pending commands")
	q := m.connection.DB("").C("command_queue").Find(bson.M{"complete": false})

	commandMessageArray := make([]irclogsme.CommandMessage, 0)
	if err := q.Iter().All(&commandMessageArray); err != nil {
		return nil, err
	}

	return commandMessageArray, nil
}

func (m *MongoDatabase) CommandComplete(cmdMsg irclogsme.CommandMessage) error {
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

func (m *MockDatabase) GetConfig() (irclogsme.Config, error) {
	LogDebug("mockdb: fetching config")
	return irclogsme.Config{}, nil
}

func (m *MockDatabase) LogMessage(message irclogsme.LogMessage) error {
	return nil
}

func (m *MockDatabase) FetchPendingCommands() ([]irclogsme.CommandMessage, error) {
	return make([]irclogsme.CommandMessage, 0), nil
}

func (m *MockDatabase) CommandComplete(cmdMsg irclogsme.CommandMessage) error {
	return nil
}
