package main

import (
	"context"
	"net/http"
	"nwr/utils"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/mongo"
)

// Dummy contact definition.
type Contact struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Dummy contacts list.
var dummyContacts = []Contact{
	{ID: "4", Name: "Aespa"},
	{ID: "5", Name: "BraveGirls"},
	{ID: "6", Name: "CherryBullet"},
}

func getUserContacts(userID string) []Contact {
	_ = userID
	return dummyContacts
}

// Simple chat ID generator (for demo purposes).
// var chatIDCounter int = 100

func generateChatID() string {
	// chatIDCounter++
	// return chatIDCounter
	return utils.GenerateIntID(16)
}

// Data structures for Chat and Message.
// Added ContactID to uniquely identify a chat per contact.
type Chat struct {
	ChatID    string `json:"chat_id" bson:"chat_id"`
	ContactID string `json:"contact_id" bson:"contact_id"`
	Name      string `json:"name" bson:"name"`
	Preview   string `json:"preview" bson:"preview"`
	Deleted   bool   `json:"deleted" bson:"deleted"`
}

type Message struct {
	MessageID   string    `json:"message_id" bson:"message_id,omitempty"` // MongoDB can auto-generate an _id if needed.
	ChatID      string    `json:"chat_id" bson:"chat_id"`
	Sender      string    `json:"sender" bson:"sender"`
	Content     string    `json:"content,omitempty" bson:"content,omitempty"`
	Caption     string    `json:"caption,omitempty" bson:"caption,omitempty"`
	File        string    `json:"filename,omitempty" bson:"filename,omitempty"`
	EditHistory []string  `json:"edithistory,omitempty" bson:"edithistory,omitempty"`
	EditedAt    time.Time `json:"editedat" bson:"editedat"`
	CreatedAt   time.Time `json:"createdat" bson:"createdat"`
	Deleted     bool      `json:"deleted" bson:"deleted"`
}

// Global variables for MongoDB.
var (
	mongoClient        *mongo.Client
	db                 *mongo.Database
	chatsCollection    *mongo.Collection
	messagesCollection *mongo.Collection
)

// Global Redis client.
var redisClient *redis.Client
var ctx = context.Background()

// WebSocket upgrader configuration.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Update as necessary to check origins
		return true
	},
}
