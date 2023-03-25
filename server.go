package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Function to convert price to bucket value
func getBucket(price string) string {
	priceVal := 0
	if price != "" {
		priceVal, _ = strconv.Atoi(price)
	}
	if priceVal >= 100 {
		return "100"
	} else if priceVal >= 50 {
		return "50"
	} else if priceVal >= 10 {
		return "10"
	} else {
		return "1"
	}
}

// Map to store WebSocket connections and UUIDs
var connections = make(map[string]*websocket.Conn)

func main() {
	e := echo.New()

	// Initialize WebSocket upgrader
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all connections by default
			return true
		},
	}

	// Handler for WebSocket connections
	e.GET("/v1/ws", func(c echo.Context) error {
		// Upgrade connection to WebSocket
		conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}

		// Generate UUID for connection
		uuid, err := uuid.NewRandom()
		if err != nil {
			log.Println(err)
			return err
		}
		log.Println(fmt.Sprintf("New connection: %s", uuid.String()))

		// Store WebSocket connection and UUID in map
		connections[uuid.String()] = conn

		// Send UUID to frontend
		err = conn.WriteMessage(websocket.TextMessage, []byte(uuid.String()))
		if err != nil {
			log.Println(err)
			return err
		}

		// Continuously read messages from WebSocket
		go func() {
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					log.Println(err)
					conn.Close()
					delete(connections, uuid.String())
					break
				}
			}
		}()

		for id, _ := range connections {
			log.Println(id)
		}

		return nil
	})

	// Handler for "/v1/match" route
	e.POST("/v1/match", func(c echo.Context) error {
		// Get price information and UUID from request body
		price := c.FormValue("price")
		uuid := c.FormValue("uuid")

		// Convert price to bucket value
		bucket := getBucket(price)

		// Store request and UUID in Redis
		rdb := redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		})

		// Start background worker to check for matching requests
		go func() {
			for {
				ctx := context.Background()

				// No matching requests, add original request to Redis
				if rdb.LLen(ctx, "requests:"+bucket).Val() == 0 {
					// Add original request back to Redis since there are no more matching requests
					pushResult := rdb.LPush(ctx, "requests:"+bucket, uuid+":"+bucket)
					log.Println(fmt.Sprintf("No matching requests, adding original request back to Redis: %s", uuid+":"+bucket))
					if pushResult.Err() != nil {
						log.Println(pushResult.Err())
					}
					break
				}

				req, err := rdb.RPop(ctx, "requests:"+bucket).Result()
				if err != nil {
					log.Println(err)
					continue
				}

				// Extract UUID and request from Redis value
				parts := strings.Split(req, ":")
				if len(parts) != 2 {
					log.Printf("Invalid Redis value: %v", req)
					continue
				}
				uuid2 := parts[0]
				bucket2 := parts[1]
				log.Println(connections)

				// Check if second frontend is still connected
				if _, ok := connections[uuid2]; !ok {
					log.Println(fmt.Sprintf("Second frontend is no longer connected: %s", uuid2))
					continue
				}

				// Send matching result to second frontend
				err = connections[uuid2].WriteMessage(websocket.TextMessage, []byte(bucket2))
				if err != nil {
					log.Println(err)
					continue
				}
				log.Println(fmt.Sprintf("Matched request: %s", bucket2))

				// Check if first frontend is still connected
				if _, ok := connections[uuid]; !ok {
					log.Println(fmt.Sprintf("First frontend is no longer connected: %s", uuid2))
					continue
				}

				// Send matching result to first frontend
				err = connections[uuid].WriteMessage(websocket.TextMessage, []byte(bucket))
				if err != nil {
					log.Println(err)
					continue
				}
				log.Println(fmt.Sprintf("Matched request: %s", bucket))
				break
			}
		}()

		// Return success response
		return c.String(http.StatusOK, "OK")
	})

	// Add CORS middleware
	e.Use(middleware.CORS())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
