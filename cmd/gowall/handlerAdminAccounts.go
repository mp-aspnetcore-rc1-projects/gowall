package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"encoding/json"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"gopkg.in/mgo.v2"
	//"strings"
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

	// don't like it. User and admin don't have it. [drywall bad]
	filters := Result["filters"].(gin.H)
	filters["search"] = c.Query("search")
	filters["status"] = c.Query("status")

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

/*


func createAdministrator(c *gin.Context) {
	response := responseAdmin{}
	response.Init(c)
	admin := getAdmin(c)

	// validate
	ok := admin.IsMemberOf("root")
	if !ok {
		response.Errors = append(response.Errors, "You may not create administrators")
		response.Fail()
		return
	}

	response.Admin.DecodeRequest(c)
	if len(response.Name.Full) == 0 {
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
	err := collection.Find(bson.M{"name.full": response.Name.Full}).One(nil)
	// we expect err == mgo.ErrNotFound for success
	if err == nil {
		response.Errors = append(response.Errors, "That administrator already exists.")
		response.Fail()
		return
	} else if err != mgo.ErrNotFound {
		panic(err)
	}

	// handleName
	name := strings.Split(response.Name.Full, " ")
	response.Name.First = name[0]
	if len(name) == 2 {
		response.Name.Last = name[1]
		response.Admin.Name.Middle = ""
	}
	if len(name) == 3 {
		response.Name.Middle = name[2]
	}
	// todo maybe when we create first root user we lose it
	response.Admin.Search = []string{response.Name.First, response.Name.Middle, response.Name.Last}
	response.Admin.Permissions = []Permission{}
	response.Admin.Groups = []string{}

	// createAdministrator
	response.Admin.ID = bson.NewObjectId()
	err = collection.Insert(response.Admin) // todo I think mgo's behavior isn't expected
	if err != nil {
		panic(err)
		return
	}
	response.Data["record"] = response
	response.Finish()
	//c.JSON(http.StatusOK, gin.H{"record": response, "success": true}) // todo necessary check
}

func readAdministrator(c *gin.Context) {
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ADMINS)
	admin := Admin{}
	err := collection.FindId(bson.ObjectIdHex(c.Param("id"))).One(&admin)
	if err != nil {
		if err == mgo.ErrNotFound {
			Status404Render(c)
			return
		}
		panic(err)
	}
	json, err := json.Marshal(admin)
	if err != nil {
		panic(err)
	}
	if XHR(c) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "application/json; charset=utf-8", json)
		return
	}

	c.Set("Record", template.JS(getEscapedString(string(json))))
	c.HTML(http.StatusOK, "/admin/administrators/details/", c.Keys)
}

func updateAdministrator(c *gin.Context) {
	response := responseAdmin{}
	response.Init(c)

	err := json.NewDecoder(c.Request.Body).Decode(&response.Admin.Name)
	if err != nil {
		panic(err)
	}
	// clean errors from client
	response.CleanErrors()

	if len(response.Name.First) == 0 {
		response.Errors = append(response.Errors, "A name is required")
	}

	if len(response.Name.Last) == 0 {
		response.Errors = append(response.Errors, "A lastname is required")
	}

	if response.HasErrors() {
		response.Fail()
		return
	}

	response.Admin.Name.Full = response.Admin.Name.First + " " + response.Admin.Name.Last

	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ADMINS)

	// patchAdministrator
	err = collection.UpdateId(bson.ObjectIdHex(c.Param("id")), bson.M{
		"$set": bson.M{
			"name": response.Admin.Name,
		},
	})
	if err != nil {
		response.Errors = append(response.Errors, err.Error())
		response.Fail()
		return
	}

	response.Finish()
}

func updateAdministratorPermissions(c *gin.Context) {
	response := responseAdmin{}
	response.Init(c)
	//TODO there are not clear logic with populate of groups
	admin := getAdmin(c)

	// validate
	ok := admin.IsMemberOf("root")
	if !ok {
		response.Errors = append(response.Errors, "You may not change the permissions of admin groups.")
		response.Fail()
		return
	}

	response.Admin.DecodeRequest(c)
	response.ErrFor = map[string]string{} // in that handler it required (non standard behavior from node)
	if len(response.Permissions) == 0 {
		response.ErrFor["permissions"] = "required"
	}

	if response.HasErrors() {
		response.Fail()
		return
	}

	//patchAdmin
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ADMINS)

	err := collection.UpdateId(bson.ObjectIdHex(c.Param("id")), bson.M{
		"$set": bson.M{
			"permissions": response.Admin.Permissions,
		},
	})
	if err != nil {
		println(err.Error())
		panic(err)
	}

	response.Finish()
}

func updateAdministratorGroups(c *gin.Context) {
	response := responseAdmin{}
	response.Init(c)
	//TODO there are not clear logic with populate of groups
	admin := getAdmin(c)

	// validate
	ok := admin.IsMemberOf("root")
	if !ok {
		response.Errors = append(response.Errors, "You may not change the group memberships of admins.")
		response.Fail()
		return
	}

	response.Admin.DecodeRequest(c)
	response.ErrFor = map[string]string{} // in that handler it required (non standard behavior from node)
	if len(response.Groups) == 0 {
		response.ErrFor["groups"] = "required"
		response.Fail()
		return
	}


	//patchAdmin
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ADMINS)

	err := collection.UpdateId(bson.ObjectIdHex(c.Param("id")), bson.M{
		"$set": bson.M{
			"groups": response.Admin.Groups,
		},
	})
	if err != nil {
		println(err.Error())
		panic(err)
	}

	response.Finish()
}

func linkUser(c *gin.Context) {
	response := responseAdmin{}
	response.Init(c)

	admin := getAdmin(c)

	// validate
	ok := admin.IsMemberOf("root")
	if !ok {
		response.Errors = append(response.Errors, "You may not change the permissions of admin groups.")
		response.Fail()
		return
	}

	var req struct {
		NewUsername string `json:"newUsername"`
	}

	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		panic(err)
	}

	response.ErrFor = map[string]string{} // in that handler it required (non standard behavior from node)
	if len(req.NewUsername) == 0 {
		response.ErrFor["newUsername"] = "required"
		response.Errors = append(response.Errors, "required")
	}

	if response.HasErrors() {
		response.Fail()
		return
	}

	//verifyUser
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(USERS)
	user := User{}
	err = collection.Find(bson.M{"username": req.NewUsername}).One(&user)
	if err != nil {
		if err != mgo.ErrNotFound {
			panic(err)
		}
		response.Errors = append(response.Errors, "User not found.")
		response.Fail()
		return
	}
	id := c.Param("id")
	if user.Roles.Admin.String() == id {
		response.Errors = append(response.Errors, "User is already linked to a different admin.")
		response.Fail()
		return
	}

	// duplicateLinkCheck
	collection = db.C(ADMINS)
	err = collection.Find(
		bson.M{
			"user.id": id,
			"_id": bson.M{
				"user.id": id,
			},
		}).One(&admin) // reuse admin. If it will be used it mean that user already linked.

	if err == nil {
		response.Errors = append(response.Errors, "Another admin is already linked to that user.")
		response.Fail()
		return
	} else if err != mgo.ErrNotFound {
		panic(err)
	}

	// patchUser
	collection = db.C(USERS)
	err = collection.UpdateId(user.ID, bson.M{
		"$set": bson.M{"roles.admin": bson.ObjectIdHex(id)},
	})
	if err != nil {
		panic(err)
	}

	// patchAdministrator
	collection = db.C(ADMINS)
	err = collection.UpdateId(bson.ObjectIdHex(id), bson.M{
		"$set": bson.M{"user": bson.M{
			"id": user.ID,
			"name": user.Username,
		}},
	})

	if err != nil {
		panic(err)
	}

	// getAdminForResponse  drywall require it // todo maybe bulk?
	err = collection.FindId(bson.ObjectIdHex(id)).One(&response.Admin)

	if err != nil {
		panic(err)
	}

	response.Data["admin"] = response.Admin
	response.Finish()
}

func unlinkUser(c *gin.Context) {
	response := responseAdmin{}
	response.Init(c)

	admin := getAdmin(c)

	// validate
	ok := admin.IsMemberOf("root")
	if !ok {
		response.Errors = append(response.Errors, "You may not change the permissions of admin groups.")
		response.Fail()
		return
	}
	id := c.Param("id")
	if admin.ID.String() == id {
		response.Errors = append(response.Errors, "You may not unlink yourself from admin.")
		response.Fail()
		return
	} // todo  here is func for errors
	response.ErrFor = map[string]string{} // in that handler it required (non standard behavior from node)

	// patchUser
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(USERS)
	err := collection.Update(bson.M{"roles.admin": bson.ObjectIdHex(id)}, bson.M{
		"$set": bson.M{"roles.admin": ""},
	})
	if err != nil {
		if err != mgo.ErrNotFound {
			panic(err)
		}
		response.Errors = append(response.Errors, "User not found.")
		response.Fail()
		return
	}

	// patchAdministrator
	collection = db.C(ADMINS)
	err = collection.UpdateId(bson.ObjectIdHex(id), bson.M{
		"$set": bson.M{"user": bson.M{}},
	})

	if err != nil {
		panic(err)
	}

	response.Data["admin"] = response.Admin
	response.Finish()
}

func deleteAdministrator(c *gin.Context) {
	response := Response{}
	response.Init(c)

	// validate
	if ok := getAdmin(c).IsMemberOf("root"); !ok {
		response.Errors = append(response.Errors, "You may not delete administrators.")
		response.Fail()
		return
	}

	// deleteUser
	db := getMongoDBInstance()
	defer db.Session.Close()
	collection := db.C(ADMINS)
	err := collection.RemoveId(bson.ObjectIdHex(c.Param("id")))
	if err != nil {
		response.Errors = append(response.Errors, err.Error())
		response.Fail()
		return
	}

	response.Finish()
}
*/
