package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nwr/utils"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Utility Functions ---

func getChatMessages(chatID string, limit int64) ([]Message, error) {
	filter := bson.M{"chat_id": chatID, "deleted": false}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit)

	cur, err := messagesCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var msgs []Message
	for cur.Next(ctx) {
		var msg Message
		if err := cur.Decode(&msg); err != nil {
			log.Println("Decode message error:", err)
			continue
		}
		msgs = append(msgs, msg)
	}

	if len(msgs) == 0 {
		msgs = []Message{}
	}

	return msgs, nil
}

func saveMessage(msg Message) error {
	_, err := messagesCollection.InsertOne(ctx, msg)
	return err
}

func updateMessage(chatID, messageID string, update bson.M) error {
	filter := bson.M{"chat_id": chatID, "message_id": messageID}
	_, err := messagesCollection.UpdateOne(ctx, filter, bson.M{"$set": update})
	return err
}

// --- Handlers ---

// Fetch messages from MongoDB
func messagesHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_ = claims

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		http.Error(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	messages, err := getChatMessages(chatID, 20)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

// Send a message and store it in the database
func sendMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	err = r.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	chatID := r.FormValue("chat_id")
	content := r.FormValue("content")
	caption := r.FormValue("caption")

	if chatID == "" {
		http.Error(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	var filename string
	if file, header, err := r.FormFile("file"); err == nil {
		defer file.Close()
		filename = header.Filename
		if err := saveUploadedFile(file, filename); err != nil {
			http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	msg := Message{
		MessageID: generateMessageID(),
		ChatID:    chatID,
		Content:   content,
		Caption:   caption,
		File:      filename,
		Sender:    claims.UserID, // Replace with actual user data.
		CreatedAt: time.Now(),
	}

	if err := saveMessage(msg); err != nil {
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

// Edit a message in the database
func editMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_ = claims

	if r.Method != http.MethodPut {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChatID     string `json:"chat_id"`
		MessageID  string `json:"message_id"`
		NewContent string `json:"new_content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	update := bson.M{
		"content":   req.NewContent,
		"edited_at": time.Now(),
	}

	if err := updateMessage(req.ChatID, req.MessageID, update); err != nil {
		http.Error(w, "Failed to update message", http.StatusInternalServerError)
		return
	}

	wsMessage := struct {
		Type       string `json:"type"`
		ChatID     string `json:"chat_id"`
		MessageID  string `json:"message_id"`
		NewContent string `json:"new_content"`
	}{
		Type:       "edit",
		ChatID:     req.ChatID,
		MessageID:  req.MessageID,
		NewContent: req.NewContent,
	}
	wsBroadcast(req.ChatID, wsMessage)

	// w.WriteHeader(http.StatusNoContent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}

// Delete a message (soft delete)
func deleteMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_ = claims

	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChatID    string `json:"chat_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}
	log.Println("rdhfyer8i748547--------------", req)
	update := bson.M{"deleted": true}

	if err := updateMessage(req.ChatID, req.MessageID, update); err != nil {
		http.Error(w, "Failed to delete message", http.StatusInternalServerError)
		return
	}

	// wsMessage := struct {
	// 	Type      string `json:"type"`
	// 	ChatID    string `json:"chat_id"`
	// 	MessageID string `json:"message_id"`
	// }{
	// 	Type:      "delete",
	// 	ChatID:    req.ChatID,
	// 	MessageID: req.MessageID,
	// }
	// wsBroadcast(req.ChatID, wsMessage)

	// w.WriteHeader(http.StatusNoContent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}

// --- Helper Functions ---

func saveUploadedFile(file io.Reader, filename string) error {
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.Mkdir(uploadDir, os.ModePerm); err != nil {
			return err
		}
	}

	dstPath := fmt.Sprintf("%s/%s", uploadDir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	return err
}

func generateMessageID() string {
	// return fmt.Sprintf("%d", time.Now().UnixNano()) // Replace with a proper unique ID generator
	return utils.GenerateIntID(18)
}
