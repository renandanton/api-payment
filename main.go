package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"reflect"
	"strconv"
)

type Charge struct {
	ID        bson.ObjectId `bson:"_id" json:"id,omitempty"`
	PaymentID bson.ObjectId `bson:"payment_id" json:"payment_id"`
	Amount    string        `bson:"amount" json:"amount"`
}

type Payment struct {
	ID     bson.ObjectId `bson:"_id" json:"id,omitempty"`
	Name   string        `bson:"name" json:"name"`
	Type   string        `bson:"type" json:"type"`
	Iban   string        `bson:"iban" json:"iban"`
	Expiry string        `bson:"expiry" json:"expiry"`
	Cc     string        `bson:"cc" json:"cc"`
	Ccv    string        `bson:"ccv" json:"ccv"`
}

type ErrorResponse struct {
    Code int64 `json:code`
    Message string `json:message`
}

var session = initMongo()
var charges []Charge
var payments []Payment

func GetListCharges(w http.ResponseWriter, req *http.Request) {
	var charges []Charge
	c := session.DB("glofox").C("charges")

	err := c.Find(bson.M{}).All(&charges)
	if err != nil {
		ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "Database Error"}, http.StatusInternalServerError)
		log.Println("Failed get all charges: ", err)
		return
	}
	
	respBody, err := json.MarshalIndent(charges, "", " ")
	if err != nil {
		log.Fatal(err)
	}

	ResponseWithJSON(w, respBody, http.StatusOK)
}

func GetChargeById(w http.ResponseWriter, req *http.Request) {
	var charge Charge
	params := mux.Vars(req)
	c := session.DB("glofox").C("charges")
	
	if bson.IsObjectIdHex(params["id"]) {
		err := c.FindId(bson.ObjectIdHex(params["id"])).One(&charge)
		if err != nil {
			ErrorWithJSON(w, ErrorResponse{Code: 404, Message: "Charge  doesn't exist"}, http.StatusNotFound)
			log.Println("Failed to find charges: ", err)
			return
		}
	} else {
		ErrorWithJSON(w, ErrorResponse{Code: 404, Message: "Invalid parameter ID"}, http.StatusNotFound)
		return
	}

	respBody, err := json.MarshalIndent(charge.SelectFields("payment_id", "amount"), "", " ")
	if err != nil {
		log.Fatal(err)
	}

	ResponseWithJSON(w, respBody, http.StatusOK)
}

func CreatePayment(w http.ResponseWriter, req *http.Request) {
	 var payment Payment
	 err := json.NewDecoder(req.Body).Decode(&payment)
	 if err != nil {
		 ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "INVALID PARAMETERS"}, http.StatusBadRequest)
		 return
	}

	c := session.DB("glofox").C("payments")

	payment.ID = bson.NewObjectId()

	err = c.Insert(payment)
	if err != nil {
		if mgo.IsDup(err) {
			ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "Payment with this ID already exist"}, http.StatusBadRequest)
			return
		}
		ErrorWithJSON(w, ErrorResponse{Code: 500, Message: "Database Error"}, http.StatusInternalServerError)
		log.Println("Failed insert payment: ", err)
	}

	err = c.Find(bson.M{"_id": payment.ID}).One(&payment)
	if err != nil {
		log.Fatal(err)
	}
	respBody, _ := json.MarshalIndent(payment, "", " ")

	ResponseWithJSON(w, respBody, http.StatusOK)
}

func CreateCharge(w http.ResponseWriter, req *http.Request) {
	var payment Payment
	var charge Charge
	err := json.NewDecoder(req.Body).Decode(&charge)
	if err != nil {
		ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "INVALID PARAMETERS"}, http.StatusBadRequest)
		log.Println("Failed load json charge: ", err)
		return
	}

	p := session.DB("glofox").C("payments")
	c := session.DB("glofox").C("charges")

	err = p.Find(bson.M{"_id": charge.PaymentID}).One(&payment)
	if err != nil {
		ErrorWithJSON(w, ErrorResponse{Code: 404, Message: "Payment ID Not Found."}, http.StatusNotFound)
		log.Println("Failed find payment ID in charge collection: ", err)
		return
	}

	amount, _ := strconv.ParseFloat(charge.Amount, 64)
	switch payment.Type {
	case "cc":
		charge.Amount = strconv.FormatFloat((((amount * 10) / 100) + amount), 'f', 2, 64)
	case "dd":
		charge.Amount = strconv.FormatFloat((((amount * 7) / 100) + amount), 'f', 2, 64)
	default:
		ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "Payment Type is not accept."}, http.StatusInternalServerError)
	}

	charge.ID = bson.NewObjectId()

	err = c.Insert(charge)
	if err != nil {
		if mgo.IsDup(err) {
			ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "Charge with this ID already exist"}, http.StatusBadRequest)
			return
		}
		ErrorWithJSON(w, ErrorResponse{Code: 400, Message: "Database Error"}, http.StatusInternalServerError)
		log.Println("Failed insert payment: ", err)
	}

	err = c.Find(bson.M{"_id": charge.ID}).One(&charge)
	if err != nil {
		log.Fatal(err)
	}
	respBody, _ := json.MarshalIndent(&charge, "", " ")

	ResponseWithJSON(w, respBody, http.StatusOK)
}

func ErrorWithJSON(w http.ResponseWriter, message ErrorResponse, code int) {
	resp, _ := json.Marshal(message)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write([]byte(resp))
}

func ResponseWithJSON(w http.ResponseWriter, json []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(json)
}

func initMongo() *mgo.Session {
	session, err := mgo.Dial("db")
	if err != nil {
		panic(err)
	}

	session.SetMode(mgo.Monotonic, true)
	return session
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/payment", CreatePayment).Methods("POST")
	router.HandleFunc("/charge", CreateCharge).Methods("POST")
	router.HandleFunc("/charge", GetListCharges).Methods("GET")
	router.HandleFunc("/charge/{id}", GetChargeById).Methods("GET")

	log.Fatal(http.ListenAndServe(":9090", router))
}

func fieldSet(fields ...string) map[string]bool {
	set := make(map[string]bool, len(fields))
	for _, s := range fields {
		set[s] = true
	}
	return set
}

func (s *Charge) SelectFields(fields ...string) map[string]interface{} {
	fs := fieldSet(fields...)
	rt, rv := reflect.TypeOf(*s), reflect.ValueOf(*s)
	out := make(map[string]interface{}, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		jsonKey := field.Tag.Get("json")
		if fs[jsonKey] {
			out[jsonKey] = rv.Field(i).Interface()
		}
	}
	return out
}
