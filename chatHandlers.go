package main

import (
	"encoding/json"
	"log"
	"net/http"
	"nwr/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Handler for creating a new chat.
func createChatHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Expected payload: { "contact_id": 3 }
	var req struct {
		ContactID string `json:"contact_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userContacts := getUserContacts(claims.UserID)

	// Find the contact in the dummy contacts list.
	var selectedContact *Contact
	for _, contact := range userContacts {
		if contact.ID == req.ContactID {
			selectedContact = &contact
			break
		}
	}
	if selectedContact == nil {
		http.Error(w, "Contact not found", http.StatusNotFound)
		return
	}

	// Check if a chat already exists for this contact.
	var existingChat Chat
	err = chatsCollection.FindOne(ctx, bson.M{"contact_id": req.ContactID}).Decode(&existingChat)
	if err == nil {
		// Chat exists, so return it.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(existingChat)
		return
	} else if err != mongo.ErrNoDocuments {
		// Some other error occurred.
		http.Error(w, "Error checking existing chat: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// No existing chat found; create a new Chat struct.
	newChat := Chat{
		ChatID:    generateChatID(),
		ContactID: req.ContactID,
		Name:      selectedContact.Name,
		Preview:   "", // Optionally, set a default preview.
	}

	// Insert the new chat into MongoDB.
	_, err = chatsCollection.InsertOne(ctx, newChat)
	if err != nil {
		http.Error(w, "Failed to create chat: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newChat)
}

func chatsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_ = claims

	limit := int64(10)

	// Exclude deleted chats
	filter := bson.M{"deleted": bson.M{"$ne": true}}

	cur, err := chatsCollection.Find(ctx, filter, &options.FindOptions{Limit: &limit})
	if err != nil {
		http.Error(w, "Failed to fetch chats", http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)

	var chats []Chat
	for cur.Next(ctx) {
		var chat Chat
		if err := cur.Decode(&chat); err != nil {
			log.Println("Decode chat error:", err)
			continue
		}
		chats = append(chats, chat)
	}

	// Ensure JSON response is an empty array instead of null
	if len(chats) == 0 {
		chats = []Chat{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}

// Handler for fetching contacts from MongoDB.
func contactsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// limit := int64(10)
	// cur, err := chatsCollection.Find(ctx, bson.D{}, &options.FindOptions{Limit: &limit})
	// if err != nil {
	// 	http.Error(w, "Failed to fetch contacts", http.StatusInternalServerError)
	// 	return
	// }
	// defer cur.Close(ctx)

	// var contacts []Contact
	// for cur.Next(ctx) {
	// 	var contact Contact
	// 	if err := cur.Decode(&contact); err != nil {
	// 		log.Println("Decode contact error:", err)
	// 		continue
	// 	}
	// 	contacts = append(contacts, contact)
	// }

	contacts := getUserContacts(claims.UserID)

	if len(contacts) == 0 {
		contacts = []Contact{}
	}

	json.NewEncoder(w).Encode(contacts)
}

// // Delete a message (soft delete)
// func deleteChatHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	if r.Method != http.MethodDelete {
// 		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	chatID := ps.ByName("chatid")
// 	log.Println("Deleting chat:", chatID)

// 	update := bson.M{"$set": bson.M{"deleted": true}}

// 	if err := softDeleteChat(chatID, update); err != nil {
// 		http.Error(w, "Failed to delete chat", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(bson.M{"chat_id": chatID, "deleted": true})
// }

// func softDeleteChat(chatID string, update bson.M) error {
// 	filter := bson.M{"chat_id": chatID}
// 	_, err := chatsCollection.UpdateOne(ctx, filter, update)
// 	return err
// }

// hard delete
func deleteChatHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	_ = claims
	chatID := ps.ByName("chatid")
	log.Println("Deleting chat:", chatID)

	if err := hardDeleteChat(chatID); err != nil {
		http.Error(w, "Failed to delete chat", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bson.M{"chat_id": chatID, "deleted": true})
}

func hardDeleteChat(chatID string) error {
	filter := bson.M{"chat_id": chatID}
	_, err := chatsCollection.DeleteOne(ctx, filter)
	return err
}
