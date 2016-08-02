package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type responseAccount struct {
	Response
	Account
}

func renderAccounts(c *gin.Context) {
	query := bson.M{}

	search, ok := c.GetQuery("search")
	if ok && len(search) != 0 {
		query["search"] = bson.RegEx{
			Pattern: `^.*?` + search + `.*$`,
			Options: "i",
		}
	}

	status, ok := c.GetQuery("status")
	if ok && len(status) != 0 {
		query["status"] = status
	}

	var results []Account

	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ACCOUNTS)

	Result := getData(c, collection.Find(query), &results)

	filters := Result["filters"].(gin.H)
	filters["search"] = search
	filters["status"] = status

	Results, err := json.Marshal(Result)
	if err != nil {
		panic(err)
	}

	if XHR(c) {
		handleXHR(c, Results)
		return
	}
	c.Set("Results", template.JS(getEscapedString(string(Results))))

	var statuses []Status
	collection = db.C(STATUSES)
	err = collection.Find(nil).All(&statuses)

	// preparing for js.  Don't like it.
	// https://groups.google.com/forum/#!topic/golang-nuts/0HJoROz2TMo
	// https://play.golang.org/p/M_AoMQwtFt
	// 10 july 2016 wasn't expected
	var statusesS string = `<option value="">-- any --</option>`
	for _, status := range statuses {
		statusesS += `<option value="` + status.ID + `">` + status.Name + `</option>`
	}
	c.Set("Statuses", template.JS(statusesS))
	c.HTML(http.StatusOK, c.Request.URL.Path, c.Keys)
}

func createAccount(c *gin.Context) {
	response := responseAccount{}
	response.Init(c)

	var name_ struct {
		Name string `json:"name.full"`
	}
	err := json.NewDecoder(c.Request.Body).Decode(&name_)
	if err != nil {
		panic(err)
	}
	response.Account.Name.Full = name_.Name
	// clean errors from client

	if len(response.Account.Name.Full) == 0 {
		response.Errors = append(response.Errors, "A name is required")
	}

	if response.HasErrors() {
		response.Fail()
		return
	}

	// handleName
	response.Name.Full = slugifyName(response.Name.Full)

	// duplicateAdministrator
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ADMINS)
	err = collection.Find(bson.M{"name.full": response.Name.Full}).One(nil)
	// we expect err == mgo.ErrNotFound for success
	if err == nil {
		response.Errors = append(response.Errors, "That account already exists.")
		response.Fail()
		return
	} else if err != mgo.ErrNotFound {
		panic(err)
	}
	// handleName
	name := strings.Split(response.Name.Full, " ")
	response.Name.First = name[0]
	if len(name) > 1 {
		if len(name) == 2 {
			response.Name.Last = name[1]
			response.Account.Name.Middle = ""
		}
		if len(name) == 3 {
			response.Name.Middle = name[2]
		}
	}

	// todo maybe when we create first root user we lose it
	response.Account.Search = []string{response.Name.First, response.Name.Middle, response.Name.Last}

	// createAdministrator
	response.Account.ID = bson.NewObjectId()
	println(response.Account.ID.String())
	err = collection.Insert(response.Account) // todo I think mgo's behavior isn't expected

	if err != nil {
		println(err.Error())
		panic(err)
		return
	}
	response.Data["record"] = response
	response.Finish()
	//c.JSON(http.StatusOK, gin.H{"record": response, "success": true}) // todo necessary check
}

func readAccount(c *gin.Context) {
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ACCOUNTS)
	account := Account{}
	err := collection.FindId(bson.ObjectIdHex(c.Param("id"))).One(&account)
	if err != nil {
		if err == mgo.ErrNotFound {
			Status404Render(c)
			return
		}
		panic(err)
	}
	json, err := json.Marshal(account)
	if err != nil {
		panic(err)
	}
	if XHR(c) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "application/json; charset=utf-8", json)
		return
	}

	c.Set("Record", template.JS(getEscapedString(string(json))))

	var statuses []Status
	collection = db.C(STATUSES)
	err = collection.Find(nil).All(&statuses)

	// preparing for js.  Don't like it.
	// https://groups.google.com/forum/#!topic/golang-nuts/0HJoROz2TMo
	// https://play.golang.org/p/M_AoMQwtFt
	// 10 july 2016 wasn't expected
	var statusesS string = `<option value="">-- any --</option>`
	for _, status := range statuses {
		statusesS += `<option value="` + status.ID + `">` + status.Name + `</option>`
	}
	c.Set("Statuses", template.JS(statusesS))
	c.HTML(http.StatusOK, "/admin/accounts/details/", c.Keys)
}

func newNote(c *gin.Context) {
	user := getUser(c)
	response := responseAccount{}
	response.Init(c)

	// validate
	var body struct {
		Data string `json:"data"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	err := decoder.Decode(&body)
	if err != nil {
		panic(err)
	}
	if len(body.Data) == 0 {
		response.Errors = append(response.Errors, "Data is required.")
		response.Fail()
		return
	}

	// addNote
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ACCOUNTS)
	account := &Account{}
	err = collection.UpdateId(bson.ObjectIdHex(c.Param("id")),
		bson.M{"$push": bson.M{"notes": bson.M{
			"_id": bson.NewObjectId(),
			"data": body.Data,
			"userCreated": bson.M{
				"id": user.ID,
				"name": user.Username,
				"time": time.Now().Format(ISOSTRING),
			},
		}},
		})
	if err != nil {
		panic(err)
	}
	err = collection.FindId(bson.ObjectIdHex(c.Param("id"))).One(account)
	if err != nil {
		panic(err)
	}
	response.Data["account"] = account
	response.Finish()
}

func newStatus(c *gin.Context) {
	user := getUser(c)
	response := responseAccount{}
	response.Init(c)

	// validate
	var body struct {
		StatusID string `json:"id"`
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	err := decoder.Decode(&body)
	if err != nil {
		panic(err)
	}
	if len(body.StatusID) == 0 {
		response.Errors = append(response.Errors, "Please choose a status.")
		response.Fail()
		return
	}

	// addStatus
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ACCOUNTS)
	account := &Account{}
	statusToAdd := bson.M{
			"_id": body.StatusID,
			"name": body.Name,
			"userCreated": bson.M{
				"id": user.ID,
				"name": user.Username,
				"time": time.Now().Format(ISOSTRING),
			},
		}
	err = collection.UpdateId(bson.ObjectIdHex(c.Param("id")),
		bson.M{
			"$push": bson.M{"statusLog": statusToAdd},
			"$set": bson.M{"status": statusToAdd},
		})
	if err != nil {
		panic(err)
	}
	err = collection.FindId(bson.ObjectIdHex(c.Param("id"))).One(account)
	if err != nil {
		panic(err)
	}
	response.Data["account"] = account
	response.Finish()
}

func deleteAccount(c *gin.Context) {
	response := Response{}
	response.Init(c)

	// validate
	if ok := getAdmin(c).IsMemberOf("root"); !ok {
		response.Errors = append(response.Errors, "You may not delete accounts.")
		response.Fail()
		return
	}

	// deleteUser
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ACCOUNTS)
	err := collection.RemoveId(bson.ObjectIdHex(c.Param("id")))
	if err != nil {
		response.Errors = append(response.Errors, err.Error())
		response.Fail()
		return
	}

	response.Finish()
}
