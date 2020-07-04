package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
	"github.com/niklasfasching/gosql"
)

type config struct {
	DBFile          string
	ImportFrequency string
	VenuesURL       string
	EventsURL       string
	Debug           bool
	Address         string
}

type Server struct {
	*config
	db *gosql.DB
}

func main() {
	c := &config{}
	bytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Could not read config.json: %s", err)
	}
	err = json.Unmarshal(bytes, c)
	if err != nil {
		log.Fatalf("Could not read config.json: %s", err)
	}

	db := &gosql.DB{
		DataSourceName: c.DBFile,
		Funcs:          map[string]interface{}{"haversine": haversine},
	}
	if err := db.Open(nil); err != nil {
		log.Fatalf("Could not open db: %s", err)
	}
	s := &Server{c, db}
	s.startScheduledUrbanSportsImport()
	log.Fatal(s.Serve())
}

func (s *Server) Serve() error {
	http.Handle("/", http.FileServer(http.Dir("assets")))
	http.Handle("/assets", http.FileServer(http.Dir("assets")))
	http.Handle("/api/db", http.HandlerFunc(s.db.Handler))
	log.Println("Serving at ", s.Address)
	return http.ListenAndServe(s.Address, nil)
}

// https://rosettacode.org/wiki/Haversine_formula
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat, dLon := degreesToRadians(lat2-lat1), degreesToRadians(lon2-lon1)
	a := math.Pow(math.Sin(dLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return c * 6371 // earth radius in km
}
func degreesToRadians(d float64) float64 { return d * math.Pi / 180 }
