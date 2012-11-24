package server

import (
	"bufio"
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"errors"
	"github.com/lukegb/irclogsme"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type ErrorStruct struct {
	Error string `json:"error"`
}

type Channel struct {
	Name string `json:"name"`
}

type Network struct {
	Name     string    `json:"name"`
	Channels []Channel `json:"channels"`
}

type FullChannel struct {
	Name     string   `json:"name"`
	LogDates []string `json:"log_dates"`
}

type LogKick struct {
	Target  string
	Message string
}

type Log struct {
	Id string `json:"id"`

	Time time.Time `json:"time"`

	Nick  string `json:"nick"`
	Ident string `json:"ident"`
	Host  string `json:"host"`

	Type string `json:"type"`

	Data interface{} `json:"data"`
}

type Logs struct {
	Channel FullChannel `json:"channel"`
	Logs    []Log       `json:"logs"`
}

func jsonResponsinator(z func(r *http.Request) (interface{}, int)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, status := z(r)
		if erre, ok := resp.(error); ok {
			if erre.Error() == "not found" {
				status = 404 // :)
			}

			resp = ErrorStruct{Error: erre.Error()}
		}
		resp_w, err := json.Marshal(resp)
		if err != nil {
			resp = ErrorStruct{Error: err.Error()}
			resp_w, err = json.Marshal(resp) // if this fails too... WTF?!?
		}
		w.WriteHeader(status)
		w.Write(resp_w)
	}
}

func networkMorph(nc irclogsme.NetworkConfig) Network {
	res := Network{}
	res.Name = nc.Name
	res.Channels = make([]Channel, 0, len(nc.Channels))
	for channel_name, _ := range nc.Channels {
		res.Channels = append(res.Channels, Channel{Name: channel_name})
	}
	return res
}

func channelMorph(networkId bson.ObjectId, channelName string, db *mgo.Database) (*FullChannel, error) {
	channel := new(FullChannel)
	channel.Name = channelName

	collect := db.C("logs")
	q := collect.Find(bson.M{"networkid": networkId, "channel": channelName})
	var dates []string
	err := q.Distinct("splitdate", &dates)
	if err != nil {
		return nil, err
	}

	channel.LogDates = dates

	return channel, nil
}

func networkOk(networkName string, db *mgo.Database) (irclogsme.NetworkConfig, error) {
	result := new(irclogsme.NetworkConfig)
	err := db.C("networks").Find(bson.M{"name": networkName}).One(result)
	if err != nil {
		return *result, err
	}
	return *result, nil
}

func channelOk(channelName string, network irclogsme.NetworkConfig) bool {
	_, channelOk := network.Channels[channelName]
	return channelOk
}

func logMorph(log irclogsme.LogMessage) Log {
	res := Log{
		Id:    log.Id.String(),
		Time:  log.Time,
		Nick:  log.Nick,
		Ident: log.Ident,
		Host:  log.Host,
	}
	// now to specify
	switch log.Type {
	case irclogsme.LMT_PRIVMSG:
		res.Type = "privmsg"
		res.Data = log.Payload
	case irclogsme.LMT_NOTICE:
		res.Type = "notice"
		res.Data = log.Payload
	case irclogsme.LMT_JOIN:
		res.Type = "join"
	case irclogsme.LMT_PART:
		res.Type = "part"
		res.Data = log.Payload
	case irclogsme.LMT_TOPIC:
		res.Type = "topic"
		res.Data = log.Payload
	case irclogsme.LMT_QUIT:
		res.Type = "quit"
		res.Data = log.Payload
	case irclogsme.LMT_KICK:
		res.Type = "kick"
		lk := LogKick{Target: log.Target.(string)}
		if message, ok := log.Payload.(string); ok {
			lk.Message = message
		}
		res.Data = lk
	}
	if sdata, ok := res.Data.(string); ok {
		if !utf8.ValidString(sdata) {
			res.Data = "[invalid unicode]"
		}
	} else if sdata, ok := res.Data.(LogKick); ok {
		if !utf8.ValidString(sdata.Message) {
			lk := res.Data.(LogKick)
			lk.Message = "[invalid unicode]"
			res.Data = lk
		}
	}
	return res
}

func wsHandler(ws *websocket.Conn, networkId bson.ObjectId, channelName string, coll *mgo.Collection) {
	// get the last object id
	bufReader := bufio.NewReader(ws)
	objectId, err := bufReader.ReadString('\n')
	objectId = objectId[:len(objectId)-1]
	if err != nil {
		panic(err)
	}
	objectIdH := bson.ObjectIdHex(objectId)

	var lastLog irclogsme.LogMessage
	err = coll.Find(bson.M{"_id": objectIdH}).One(&lastLog)
	if err != nil {
		panic(err)
	}
	ws.Write([]byte("RUNNING\n"))

	// check any newer
	var logs []irclogsme.LogMessage
	timer := time.Tick(1 * time.Second)
	for {
		<-timer
		q := coll.Find(bson.M{"networkid": networkId, "channel": channelName, "time": bson.M{"$gt": lastLog.Time}}).Sort("time")
		err := q.All(&logs)
		if err != nil {
			panic(err)
		}

		for _, loga := range logs {
			log.Println(loga)
			logFormat := logMorph(loga)
			form, err := json.Marshal(logFormat)
			if err != nil {
				panic(err)
			}
			ws.Write([]byte(string(form) + "\n"))

			if loga.Time.After(lastLog.Time) {
				lastLog = loga
			}
		}
	}
}

func Start() {
	dbc, err := mgo.Dial("mongodb://localhost/irclogsme")
	if err != nil {
		log.Fatalln(err)
	}
	db := dbc.DB("")

	prefix := "/api/"
	http.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		// figure out what mode we're in
		chopped := ""
		if len(r.URL.Path) > len(prefix) {
			chopped = r.URL.Path[len(prefix):]
		}
		choppedBits := strings.Split(chopped, "/")
		slashCount := strings.Count(chopped, "/")
		if slashCount == 3 && choppedBits[2] == "ws" { // websoketz
			serverName := choppedBits[0]
			channelName := "#" + choppedBits[1]
			// fetch the network
			network, err := networkOk(serverName, db)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}

			// check if the channel's in the list
			if !channelOk(channelName, network) {
				http.Error(w, "not found", 404)
			}

			// return logs
			coll := db.C("logs")
			q := coll.Find(bson.M{"networkid": network.Id, "channel": channelName}).Sort("time")
			count, err := q.Count()
			if err != nil {
				http.Error(w, err.Error(), 500)
			} else if count == 0 {
				http.Error(w, "not found", 404)
			}

			// OK, let's go
			websocket.Handler(func(ws *websocket.Conn) { wsHandler(ws, network.Id, channelName, coll) }).ServeHTTP(w, r)
		} else if slashCount == 3 { // date, server and channel - return logs!
			jsonResponsinator(func(r *http.Request) (interface{}, int) {
				serverName := choppedBits[0]
				channelName := "#" + choppedBits[1]
				logDate := choppedBits[2]

				// fetch the network
				network, err := networkOk(serverName, db)
				if err != nil {
					return err, 500
				}

				// check if the channel's in the list
				if !channelOk(channelName, network) {
					return errors.New("not found"), 404
				}

				// return logs
				coll := db.C("logs")
				q := coll.Find(bson.M{"networkid": network.Id, "channel": channelName, "splitdate": logDate}).Sort("time")
				var qRes []irclogsme.LogMessage
				err = q.All(&qRes)
				if err != nil {
					return err, 500
				}

				if len(qRes) == 0 {
					return errors.New("not found"), 404
				}

				res := make([]Log, len(qRes))
				for k, v := range qRes {
					res[k] = logMorph(v)
				}

				var response Logs
				response.Logs = res
				fullChan, err := channelMorph(network.Id, channelName, db)
				if err != nil {
					return err, 500
				}
				response.Channel = *fullChan

				return response, 200
			})(w, r)
		} else if slashCount == 2 { // server and channel passed?
			jsonResponsinator(func(r *http.Request) (interface{}, int) {
				serverName := choppedBits[0]
				channelName := "#" + choppedBits[1]

				// fetch the network
				network, err := networkOk(serverName, db)
				if err != nil {
					return err, 500
				}

				// check if the channel's in the list
				if !channelOk(channelName, network) {
					return errors.New("not found"), 404
				}

				// checks out OK, channelmorph
				res, err := channelMorph(network.Id, channelName, db)
				if err != nil {
					return err, 500
				}

				return res, 200
			})(w, r)
		} else if slashCount == 1 { // server passed?
			jsonResponsinator(func(r *http.Request) (interface{}, int) {
				serverName := choppedBits[0]
				// perform lookup
				result := new(irclogsme.NetworkConfig)
				err := db.C("networks").Find(bson.M{"name": serverName}).One(result)
				if err != nil {
					return err, 500
				}
				true_result := networkMorph(*result)
				return true_result, 200
			})(w, r)
		} else if slashCount == 0 {
			jsonResponsinator(func(r *http.Request) (interface{}, int) {
				result := make([]irclogsme.NetworkConfig, 0)
				err := db.C("networks").Find(bson.M{}).All(&result)
				if err != nil {
					return err, 500
				}

				true_result := make([]Network, len(result))
				for n, val := range result {
					true_result[n] = networkMorph(val)
				}
				return true_result, 200
			})(w, r)
		} else {
			jsonResponsinator(func(r *http.Request) (interface{}, int) {
				return errors.New("not found"), 400
			})(w, r)
		}
	})

	log.Fatalln(http.ListenAndServe(":5022", nil))
}
