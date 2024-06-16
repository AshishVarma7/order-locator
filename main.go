package main

import (
	"context"
	"log"
	"net/http"
	"text/template"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Order struct to represent order data
type Order struct {
	Name                   string `bson:"name"`
	Phone                  string `bson:"phone"`
	Address                string `bson:"address"`
	PreferableDeliveryTime string `bson:"preferable_delivery_time"`
}

var client *mongo.Client
var ordersCollection *mongo.Collection

func main() {
	// Initialize MongoDB client
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	var err error
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	// Initialize collection
	ordersCollection = client.Database("order_db").Collection("orders")

	// Define HTTP handlers
	http.HandleFunc("/", formHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/map", mapHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start server
	log.Println("Server listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// formHandler handles rendering the form HTML page
func formHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/form.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// submitHandler handles form submission and stores data in MongoDB
func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract form values
	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")
	preferableDeliveryTime := r.FormValue("preferable_delivery_time")

	// Create Order object
	order := Order{
		Name:                   name,
		Phone:                  phone,
		Address:                address,
		PreferableDeliveryTime: preferableDeliveryTime,
	}

	// Insert into MongoDB
	_, err = ordersCollection.InsertOne(context.Background(), order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect to a success page or another route
	http.Redirect(w, r, "/map", http.StatusSeeOther)
}

// mapHandler handles rendering the map HTML page
func mapHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch orders from MongoDB
	cursor, err := ordersCollection.Find(context.Background(), bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var orders []Order
	for cursor.Next(context.Background()) {
		var order Order
		if err := cursor.Decode(&order); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		orders = append(orders, order)
	}
	if err := cursor.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render map page with orders data
	tmpl, err := template.ParseFiles("templates/map.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, orders)
}
