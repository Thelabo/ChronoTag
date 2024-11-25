package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var rb *redis.Client

type Version struct {
	Version string `json:"version"`
	Major   int    `json:"major"`
	Minor   int    `json:"minor"`
}

func (v *Version) Increment() {
	v.Minor++
}

func (v *Version) IncrementMajor() {
	v.Major++
	v.Minor = 0
}

func (v *Version) String() string {
	v.Version = fmt.Sprintf("%d.%d", v.Major, v.Minor)
	return v.Version
}

func ParseVersion(version string) Version {

	var v Version
	v.Version = version

	_, err := fmt.Sscanf(version, "%d.%d", &v.Major, &v.Minor)

	if err != nil {
		fmt.Printf("Error parsing version: %v", err)
	}

	return v

}

func initRedis() {

	addr := os.Getenv("REDIS_ADDR")

	if addr == "" {
		addr = "redis:6379"
	}

	rb = redis.NewClient(&redis.Options{
		Addr: addr,
	})

	_, err := rb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Error connecting to redis: %v", err)
	}

	log.Printf("Connected to redis successful")

}

func getVersionHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	branch := r.URL.Query().Get("branch")

	type Response struct {
		Version string `json:"version"`
	}

	fmt.Printf("Getting version for target")

	if target == "" || branch == "" {
		http.Error(w, "target and branch are required fdff", http.StatusBadRequest)
		return
	}
	combinedKey := target + ":" + branch

	if rb == nil {
		http.Error(w, "Failed to get connection to redis", http.StatusInternalServerError)
	}

	version, err := rb.Get(ctx, combinedKey).Result()

	if err == redis.Nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "Error getting version", http.StatusInternalServerError)
		return
	}

	resp := Response{Version: version}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

}

func incrementVersionHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	branch := r.URL.Query().Get("branch")

	if target == "" || branch == "" {
		http.Error(w, "target and branch are required", http.StatusBadRequest)
		return
	}

	branchTarget := target + ":" + branch

	lastVersion, err := rb.Get(ctx, branchTarget).Result()

	if err == redis.Nil {
		lastVersion = "0"
	}

	lastVersionInt, err := strconv.Atoi(lastVersion)

	if err != nil {
		http.Error(w, "Error converting version to integer", http.StatusInternalServerError)
		return
	}
	newVersion := lastVersionInt + 1

	err = rb.Set(ctx, branchTarget, newVersion, 0).Err()

	if err != nil {
		http.Error(w, "Error setting version", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": strconv.Itoa(newVersion)})

}

func main() {
	initRedis()
	http.HandleFunc("/version", getVersionHandler)
	http.HandleFunc("/increment", incrementVersionHandler)
	log.Println("Starting server on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))

}
