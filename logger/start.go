package logger

import (
	"flag"
	irc "github.com/fluffle/goirc/client"
	"github.com/lukegb/irclogsme"
	"io/ioutil"
	"labix.org/v2/mgo/bson"
	"time"
)

const (
	VERSION_STRING = "0.1"
	VERSION_ID     = 1

	MULTIPLEXER_INTERVAL = 1 * time.Second
	MULTIPLEXER_TIMEOUT  = 60 * time.Second
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

func ircClientRoutine(netConf irclogsme.NetworkConfig, messageChan chan irclogsme.LogMessage, cmdChan chan irclogsme.CommandMessage) {
	// this is a go routine
	ircCli := irc.SimpleClient(netConf.Nick, netConf.User, "http://irclogs.me")
	quit := make(chan bool)
	ircCli.AddHandler("disconnected", func(conn *irc.Conn, line *irc.Line) {
		LogInfo("(%s) Disconnected?!?", netConf.Name)
		quit <- true
	})

	ircCli.AddHandler("connected", func(conn *irc.Conn, line *irc.Line) {
		LogInfo("(%s) Connected!")
		LogInfo("(%s) Executing connection commands.")
		for _, cmd := range netConf.AuthCommands {
			LogDebug("(%s) - executing: %s", netConf.Name, cmd)
			conn.Raw(cmd)
		}
		LogInfo("(%s) Joining channels.", netConf.Name)
		for channelName, _ := range netConf.Channels {
			LogDebug("(%s) - joining %s", netConf.Name, channelName)
			conn.Join(channelName)
			time.Sleep(1 * time.Second)
		}
	})

	ircCli.AddHandler("PRIVMSG", func(conn *irc.Conn, line *irc.Line) {
		LogDebug("(%s) [%s] <%s> {%s} %s", netConf.Name, line.Time.String(), line.Src, line.Args[0], line.Args[1])
		// make a log message!
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_PRIVMSG,
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
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_NOTICE,
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
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_TOPIC,
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
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_JOIN,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("PART", func(conn *irc.Conn, line *irc.Line) {
		var message string
		if len(line.Args) > 1 {
			message = line.Args[1]
		}
		LogDebug("(%s) [%s] <%s> parted %s: %s", netConf.Name, line.Time.String(), line.Src, line.Args[0], message)
		// make a log message!
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_PART,
			NetworkId: netConf.Id,
			Channel:   line.Args[0],
			Payload:   message,
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("KICK", func(conn *irc.Conn, line *irc.Line) {
		var message string
		if len(line.Args) > 2 {
			message = line.Args[2]
		}
		channel := line.Args[0]
		who := line.Args[1]
		LogDebug("(%s) [%s] <%s> kicked %s from %s: %s", netConf.Name, line.Time.String(), line.Src, who, channel, message)
		// make a log message!
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_PART,
			NetworkId: netConf.Id,
			Channel:   channel,
			Payload:   message,
			Target:    who,
			Time:      line.Time,
			Nick:      line.Nick,
			Ident:     line.Ident,
			Host:      line.Host,
		}
	})

	ircCli.AddHandler("QUIT", func(conn *irc.Conn, line *irc.Line) {
		var message string
		if len(line.Args) > 0 {
			message = line.Args[0]
		}
		LogDebug("(%s) [%s] <%s> quit: %s", netConf.Name, line.Time.String(), line.Src, message)
		// make a log message!
		messageChan <- irclogsme.LogMessage{
			Type:      irclogsme.LMT_QUIT,
			NetworkId: netConf.Id,
			Payload:   message,
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

		superloop := true
		for superloop {
			select {
			case <-quit:
				break
			case cmdmsg := <-cmdChan:
				LogDebug("(%s) Got Command: %x", netConf.Name, cmdmsg)
				switch cmdmsg.Type {
				case irclogsme.CMT_CONNECT:
					LogDebug("(%s) connect unimplemented!", netConf.Name)
				case irclogsme.CMT_DISCONNECT:
					LogDebug("(%s) disconnecting...")
					ircCli.Quit("disconnecting...")
					// waiting for next command
					for {
						cmdmsg := <-cmdChan
						LogDebug("(%s) Got Command while d/ced: %x", netConf.Name, cmdmsg)
						if cmdmsg.Type == irclogsme.CMT_CONNECT {
							LogInfo("(%s) Reconnecting...", netConf.Name)
							superloop = false
							break
						} else {
							LogInfo("(%s) Command dropped!")
						}
					}
				case irclogsme.CMT_START_LOGGING:
					LogDebug("(%s) joining channel %s", netConf.Name, cmdmsg.Channel)
					ircCli.Join(cmdmsg.Channel)
				case irclogsme.CMT_STOP_LOGGING:
					LogDebug("(%s) parting channel %s", netConf.Name, cmdmsg.Channel)
					ircCli.Part(cmdmsg.Channel, "told to part")
				case irclogsme.CMT_TELL:
					LogDebug("(%s) telling <%s> %s", netConf.Name, cmdmsg.Target, cmdmsg.Message)
					ircCli.Privmsg(cmdmsg.Target, cmdmsg.Message)
				}
			}
		}

		time.Sleep(1 * time.Second)
		currentServer = (currentServer + 1) % len(netConf.IrcServers)
	}
}

func commandMultiplexer(db Database, netMap map[bson.ObjectId]chan irclogsme.CommandMessage) {
	LogInfo("Command multiplexer starting - interval %x", MULTIPLEXER_INTERVAL)
	multiplexerTicker := time.Tick(MULTIPLEXER_INTERVAL)
	for {
		LogDebug("CMDMX - running tick")

		LogDebug("CMDMX - fetching commands from database")
		c, err := db.FetchPendingCommands()
		if err != nil {
			LogDebug("CMDMX - got error %x", err)
		} else {
			for _, cmd := range c {
				LogDebug("CMDMX - command: %x", cmd)
				timeOut := time.After(MULTIPLEXER_TIMEOUT)
				select {
				case netMap[cmd.NetworkId] <- cmd:
					LogDebug("CMDMX - command sent!")
					LogDebug("CMDMX - setting command as complete:")
					if err := db.CommandComplete(cmd); err != nil {
						LogDebug("CMDMX - command error! %x", err)
						continue
					}
					LogDebug("CMDMX - command marked complete.")
				case <-timeOut:
					LogDebug("CMDMX - command timeout!")
				}
			}
		}

		<-multiplexerTicker
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

	messageChan := make(chan irclogsme.LogMessage, 20)

	netMap := make(map[bson.ObjectId]chan irclogsme.CommandMessage)

	// well, here goes!
	for _, net := range config.Networks {
		cmdChan := make(chan irclogsme.CommandMessage)
		go ircClientRoutine(net, messageChan, cmdChan)
		netMap[net.Id] = cmdChan
	}

	go func(mChan chan irclogsme.LogMessage, db Database) {
		for {
			message := <-mChan
			err := db.LogMessage(message)

			if err != nil {
				LogError("error saving message to DB!", err)
			}
		}
	}(messageChan, db)

	go commandMultiplexer(db, netMap)

	// spin
	<-make(chan int)

}
