package logger

import (
	"flag"
	irc "github.com/fluffle/goirc/client"
	"io/ioutil"
	"time"
)

const (
	VERSION_STRING = "0.1"
	VERSION_ID     = 1
)

var (
	DB_CONN_STRING = flag.String("db_string", "", "defines a full mgo DB connection string")
	DB_CONN_FILE   = flag.String("db_file", "db.config", "defines which file the mgo DB connection string should be read from - ignored if db_string set")
)

func readStringFromFile(filename string) (string, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(f), nil
}

func ircClientRoutine(netConf NetworkConfig, messageChan chan LogMessage) {
	// this is a go routine
	ircCli := irc.SimpleClient(netConf.Nick, netConf.User, "http://irclogs.me")
	quit := make(chan bool)
	ircCli.AddHandler("disconnected", func(conn *irc.Conn, line *irc.Line) {
		LogInfo("(%s) Disconnected?!?", netConf.Name)
		quit <- true
	})

	ircCli.AddHandler("connected", func(conn *irc.Conn, line *irc.Line) {
		LogInfo("(%s) Connected! Joining channels.", netConf.Name)
		for channelName, _ := range netConf.Channels {
			LogDebug("(%s) - joining %s", netConf.Name, channelName)
			conn.Join(channelName)
			time.Sleep(1 * time.Second)
		}
	})

	ircCli.AddHandler("PRIVMSG", func(conn *irc.Conn, line *irc.Line) {
		LogDebug("(%s) [%s] <%s> {%s} %s", netConf.Name, line.Time.String(), line.Src, line.Args[0], line.Args[1])
		// make a log message!
		messageChan <- LogMessage{
			Type:      LMT_PRIVMSG,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Payload:   line.Args[1],
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("NOTICE", func(conn *irc.Conn, line *irc.Line) {
		LogDebug("(%s) [%s] <%s> {%s} %s", netConf.Name, line.Time.String(), line.Src, line.Args[0], line.Args[1])

		if line.Args[0][0] != '#' {
			return
		}

		// make a log message!
		messageChan <- LogMessage{
			Type:      LMT_NOTICE,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Payload:   line.Args[1],
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("TOPIC", func(conn *irc.Conn, line *irc.Line) {
		LogDebug("(%s) [%s] <%s> {%s} %s", netConf.Name, line.Time.String(), line.Src, line.Args[0], line.Args[1])
		// make a log message!
		messageChan <- LogMessage{
			Type:      LMT_TOPIC,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Payload:   line.Args[1],
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("JOIN", func(conn *irc.Conn, line *irc.Line) {
		LogDebug("(%s) [%s] <%s> joined %s", netConf.Name, line.Time.String(), line.Src, line.Args[0])
		// make a log message!
		messageChan <- LogMessage{
			Type:      LMT_JOIN,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("PART", func(conn *irc.Conn, line *irc.Line) {
		LogDebug("(%s) [%s] <%s> parted %s: %s", netConf.Name, line.Time.String(), line.Src, line.Args[0], line.Args[1])
		// make a log message!
		messageChan <- LogMessage{
			Type:      LMT_PART,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Payload:   line.Args[1],
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	currentServer := 0

	LogInfo("(%s) starting loop", netConf.Name)
	for {
		LogInfo("(%s) CONNECTING", netConf.Name)
		ircCli.Connect(netConf.IrcServers[currentServer])
		<-quit

		time.Sleep(1 * time.Second)
		currentServer = (currentServer + 1) % len(netConf.IrcServers)
	}
}

func Start() {
	LogInfo("Starting up irclogs.me logger v%s (version identifier: %d)", VERSION_STRING, VERSION_ID)
	flag.Parse()

	var db Database
	var err error
	var dbConnString string
	if *DB_CONN_STRING == "" {
		if dbConnString, err = readStringFromFile(*DB_CONN_FILE); err != nil {
			LogFatal("unable to load db connection string from file %s - %s", *DB_CONN_FILE, err.Error())
		}
	} else {
		dbConnString = *DB_CONN_STRING
	}
	if db, err = DbConnect(dbConnString); err != nil {
		LogFatal("failed to connect to database - %s", err.Error())
	}

	config, err := db.GetConfig()
	if err != nil {
		LogFatal("failed to get config from database - %s", err.Error())
	}

	messageChan := make(chan LogMessage, 20)

	// well, here goes!
	for _, net := range config.Networks {
		go ircClientRoutine(net, messageChan)
	}

	go func(mChan chan LogMessage, db Database) {
		for {
			message := <-mChan
			err := db.LogMessage(message)

			if err != nil {
				LogError("error saving message to DB!", err)
			}
		}
	}(messageChan, db)

	// spin
	<-make(chan int)

}
