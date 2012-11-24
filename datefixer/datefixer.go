package main

import (
	"github.com/lukegb/irclogsme"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
)

func main() {
	var err error
	dbc, err := mgo.Dial("mongodb://localhost/irclogsme")
	if err != nil {
		log.Fatalln(err)
	}
	db := dbc.DB("")

	// query all logs
	log.Println("querying logs!")

	q := db.C("logs")
	count, err := q.Count()
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("%d logs in DB", count)

	// fix dem dates
	i := q.Find(bson.M{}).Iter()
	var logEntry irclogsme.LogMessage
	doneC := 0
	for i.Next(&logEntry) {
		logEntry.SplitDate = logEntry.Time.Format("2006-01-02")

		q.Update(bson.M{"_id": logEntry.Id}, bson.M{"$set": bson.M{"splitdate": logEntry.SplitDate}})

		doneC += 1
		if doneC%10 == 0 {
			log.Printf("%d / %d - %d%%", doneC, count, (doneC*100)/count)
		}
	}
	if i.Err() != nil {
		log.Fatalln(i.Err())
	}
}
