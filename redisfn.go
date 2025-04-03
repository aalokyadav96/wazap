package main

import (
	"encoding/json"
	"log"
	"time"
)

// Flush messages from Redis to MongoDB in bulk.
func flushRedisMessages() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		// Get all keys matching chat:*:messages.
		keys, err := redisClient.Keys(ctx, "chat:*:messages").Result()
		if err != nil {
			log.Println("Redis scan error:", err)
			continue
		}
		for _, key := range keys {
			// Retrieve all messages from Redis.
			msgs, err := redisClient.LRange(ctx, key, 0, -1).Result()
			if err != nil {
				log.Println("Redis LRange error:", err)
				continue
			}
			if len(msgs) == 0 {
				continue
			}
			var messagesBulk []interface{}
			for _, mStr := range msgs {
				var m Message
				if err := json.Unmarshal([]byte(mStr), &m); err != nil {
					log.Println("JSON unmarshal error:", err)
					continue
				}
				messagesBulk = append(messagesBulk, m)
			}
			if len(messagesBulk) > 0 {
				_, err := messagesCollection.InsertMany(ctx, messagesBulk)
				if err != nil {
					log.Println("MongoDB InsertMany error:", err)
					continue
				}
				// Remove the key from Redis after successful insertion.
				redisClient.Del(ctx, key)
			}
		}
	}
}
