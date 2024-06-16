package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"text/template"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Order represents an order with name and address
type Order struct {
	Name                   string `bson:"name"`
	Phone                  string `bson:"phone"`
	Address                string `bson:"address"`
	PreferableDeliveryTime string `bson:"preferable_delivery_time"`
}

var ordersCollection *mongo.Collection

func main() {
	// Initialize MongoDB client
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// Ping the MongoDB server
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Set up collection
	ordersCollection = client.Database("order_db").Collection("orders")

	// Define HTTP handlers
	http.HandleFunc("/", formHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/map", mapHandler)
	http.HandleFunc("/api/orders", ordersAPIHandler) // Endpoint for orders JSON data
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}
	log.Printf("Server listening on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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

	// Redirect to the map page
	http.Redirect(w, r, "/map", http.StatusSeeOther)
}

// mapHandler handles rendering the map HTML page
func mapHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/map.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// ordersAPIHandler handles returning JSON data of orders and locations
func ordersAPIHandler(w http.ResponseWriter, r *http.Request) {
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

	// Geocode addresses to get locations
	var locations []map[string]float64
	for _, order := range orders {
		location, err := geocodeAddress(order.Address)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		locations = append(locations, location)
	}

	// Create a data structure for JSON response
	data := struct {
		Orders    []Order
		Locations []map[string]float64
	}{
		Orders:    orders,
		Locations: locations,
	}

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set content type and send response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// geocodeAddress fetches geocoding data from Google Maps API
func geocodeAddress(address string) (map[string]float64, error) {
	apiKey := "AIzaSyA1Rz_xGPNYMO7WyP1wYdVzVoMOCO_UUtQ"
	geocodeURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?address=%s&key=%s", url.QueryEscape(address), apiKey)

	resp, err := http.Get(geocodeURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "OK" {
		return nil, fmt.Errorf("Geocode request failed with status: %s", result.Status)
	}

	location := map[string]float64{
		"lat": result.Results[0].Geometry.Location.Lat,
		"lng": result.Results[0].Geometry.Location.Lng,
	}
	return location, nil
}
