package main

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
)

type Note struct {
	ID          bson.ObjectId `bson:"_id,omitempty"`
	Data        string `bson:"data"`
	UserCreated struct {
					ID   mgo.DBRef `bson:"id"`
					Name string `bson:"name"`
					Time time.Time `bson:"time"`
				} `bson:"userCreated"`
}