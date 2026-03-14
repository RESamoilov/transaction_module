package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type TransactionEvent struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Type      string  `json:"transaction_type"`
	Timestamp string  `json:"timestamp"`
}

const (
	kafkaBroker = "localhost:9094"
	topic       = "casino.transactions.v1"
)

func main() {
	// Настраиваем Kafka Writer
	writer := &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    500,
		BatchTimeout: 10 * time.Millisecond,
		Async:        false,
	}
	defer writer.Close()

	http.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("Starting load test for 5 seconds...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var totalSent int64
		var wg sync.WaitGroup

		workers := 20
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					txType := "bet"
					if rand.Intn(2) == 1 {
						txType = "win"
					}

					amount := math.Round((rand.Float64()*999.99+0.01)*100) / 100

					event := TransactionEvent{
						ID:        uuid.New().String(),
						UserID:    fmt.Sprintf("user-%d", rand.Intn(1000)),
						Amount:    amount,
						Type:      txType,
						Timestamp: time.Now().UTC().Format(time.RFC3339),
					}

					payload, _ := json.Marshal(event)

					// Отправляем в Kafka (с ключом по UserID для сохранения порядка, если нужно)
					err := writer.WriteMessages(context.Background(), kafka.Message{
						Key:   []byte(event.UserID),
						Value: payload,
					})

					if err != nil {
						log.Printf("Worker %d failed to write message: %v\n", workerID, err)
					} else {
						atomic.AddInt64(&totalSent, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		msg := fmt.Sprintf("Load test completed! Successfully sent %d transactions to Kafka in 5 seconds.\n", totalSent)
		log.Print(msg)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(msg))
	})

	log.Println("Load Tester is running on http://localhost:8081")
	log.Println("Send a POST request to http://localhost:8081/load to trigger the DDOS.")

	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
