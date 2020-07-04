package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/niklasfasching/gosql"
)

var importFrequency = 7 * 24 * time.Hour

var query = `
DROP TABLE IF EXISTS venues;
CREATE TABLE venues (
    ID PRIMARY KEY,
    Name,
    District,
    PostalCode,
    Address,
    Lat,
    Lon,
    Plans
);

DROP TABLE IF EXISTS events;
CREATE TABLE events (
    ID PRIMARY KEY,
    Name,
    Date,
    StartTime,
    EndTime,
    VenueID,
    CategoryID,
    CategoryName,
    Plans,
    Type,
    ServiceType,
    BookingType
);`

func (s *Server) startScheduledUrbanSportsImport() {
	go func() {
		for {
			if err := s.urbanSportsImport(); err != nil {
				panic(err)
			}
			<-time.After(time.Hour)
		}
	}()
}

func (s *Server) urbanSportsImport() error {
	timestamp, err := s.db.GetVersion()
	if err != nil {
		return err
	}
	if then := time.Unix(int64(timestamp), 0); time.Since(then) < importFrequency {
		log.Printf("Skipping import: Time since last import (%s) < import frequency (%s)", time.Since(then), importFrequency)
		return nil
	} else {
		log.Printf("Starting import: Time since last import (%s): %s. Import frequency: %s", then, time.Since(then), importFrequency)
	}

	tx, err := s.db.Begin()
	if _, err := gosql.Exec(tx, query); err != nil {
		return err
	}
	if err := venues(tx, s.VenuesURL); err != nil {
		return err
	}
	if err := events(tx, s.EventsURL); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.db.SetVersion(int(time.Now().Unix()))
}

func venues(c gosql.Connection, url string) error {
	rs := []*venueResponse{}
	if err := fetchAll(url, &rs); err != nil {
		return err
	}
	n := 0
	for _, r := range rs {
		n += len(r.Data)
		for _, v := range r.Data {
			m := map[string]interface{}{
				"ID":         v.ID,
				"Name":       v.Name,
				"Address":    v.Location.Address,
				"District":   v.Location.District.Name,
				"PostalCode": v.Location.PostalCode,
				"Lat":        v.Location.Latitude,
				"Lon":        v.Location.Longitude,
				"Plans":      v.Plans,
			}
			if _, err := gosql.Insert(c, "venues", m, true); err != nil {
				return err
			}
		}
	}
	log.Printf("inserting %d venues", n)
	return nil
}

func events(c gosql.Connection, url string) error {
	rs := []*eventResponse{}
	if err := fetchAll(url, &rs); err != nil {
		return err
	}
	prep := func(e event, t string) map[string]interface{} {
		return map[string]interface{}{
			"ID":           e.ID,
			"Name":         e.Name,
			"Date":         e.Date,
			"StartTime":    e.StartTime,
			"EndTime":      e.EndTime,
			"VenueID":      e.Venue.ID,
			"CategoryID":   e.Category.ID,
			"CategoryName": e.Category.Name,
			"Plans":        e.Plans,
			"Type":         t,
			"ServiceType":  e.ServiceType,
			"BookingType":  e.BookingType,
		}
	}

	nc, nf := 0, 0
	for _, r := range rs {
		nc, nf = nc+len(r.Data.Classes), nf+len(r.Data.FreeTrainings)
		for _, e := range r.Data.Classes {
			if _, err := gosql.Insert(c, "events", prep(e, "class"), true); err != nil {
				return err
			}
		}
		for _, e := range r.Data.FreeTrainings {
			if _, err := gosql.Insert(c, "events", prep(e, "free"), true); err != nil {
				return err
			}
		}
	}
	log.Printf("inserting %d classes and %d free training", nc, nf)
	return nil
}

type venue struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Location struct {
		District struct {
			Name string `json:"name"`
		} `json:"district"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		PostalCode string  `json:"postalCode"`
		Address    string  `json:"address"`
	} `json:"location"`
	Plans  []string `json:"planTypes"`
	Covers []struct {
		Cover311 string `json:"cover311"`
	} `json:"covers"`
	Deleted int `json:"deleted"`
}

type event struct {
	ID        int    `json:"id"`
	Name      string `json:"title"`
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Venue     struct {
		ID int `json:"id"`
	}
	Category struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"category"`
	Plans       []string `json:"planTypes"`
	External    bool     `json:"external"`
	ServiceType string   `json:"serviceType"`
	BookingType string   `json:"bookingType"`
	Deleted     int      `json:"deleted"`
}

type eventResponse struct {
	Success string `json:"success"`
	Data    struct {
		Classes       []event `json:"classes"`
		FreeTrainings []event `json:"freeTrainings"`
	} `json:"data"`
}

type venueResponse struct {
	Success string  `json:"success"`
	Data    []venue `json:"data"`
}

type response interface{ Empty() bool }

func (r *venueResponse) Empty() bool { return len(r.Data) == 0 }
func (r *eventResponse) Empty() bool { return len(r.Data.Classes)+len(r.Data.FreeTrainings) == 0 }

func fetch(url string, v interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		// the urban sports api is broken and returns an empty array for data when reaching the end
		// even for courses - it should be data: {classes: [], freeTrainings: []}... whatever
		if string(body) == `{"success":"true","data":[]}` {
			return nil
		}
		log.Printf("%T %s %s", v, url, body)
		return err
	}
	return nil
}

func fetchAll(url string, slicePtr interface{}) error {
	vs, t := reflect.ValueOf(slicePtr).Elem(), reflect.TypeOf(slicePtr).Elem().Elem().Elem()
	for i := 1; ; i++ {
		v, currentURL := reflect.New(t), fmt.Sprintf("%s&page=%d", url, i)
		r := v.Interface().(response)
		if err := fetch(currentURL, r); err != nil {
			log.Println(currentURL, err)
			return err
		}
		vs.Set(reflect.Append(vs, v))
		if r.Empty() {
			break
		}
		log.Printf("fetchAll %s %d (%s)", t, i, currentURL)
	}
	return nil
}
