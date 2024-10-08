package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	subjectSub                = "coffee.web.requests"
	subjectStockManagerPrefix = "coffee.stock"
	coffeeMakersSubjectPrefix = "coffee.orders"
)

type controllerResponse struct {
	Status  string `json:"status"` // success or error
	Message string `json:"message"`
}

type CoffeeOrder struct {
	Size       string `json:"size"`
	BeanType   string `json:"bean_type"`
	Milk       string `json:"milk"`
	Name       string `json:"name"`
	SugarCount string `json:"sugar_count"`
}

func main() {

	url := os.Getenv("NATS_URL")
	if url == "" {
		url = "192.168.128.51:4222"
	}

	coffeeQuantityMap := make(map[string]int)
	coffeeQuantityMap["small"] = 9
	coffeeQuantityMap["medium"] = 17
	coffeeQuantityMap["large"] = 30

	opt, err := nats.NkeyOptionFromSeed("seed.txt")
	if err != nil {
		log.Fatal(err)
	}

	nc, err := nats.Connect(os.Getenv("NATS_URL"), opt)
	if err != nil {
		log.Fatal("connect to nats: ", err)
	}

	defer nc.Drain()

	sub, err := nc.Subscribe(subjectSub, func(msg *nats.Msg) {

		var order CoffeeOrder

		log.Println("☕ Received a new order: " + string(msg.Data))

		err := json.Unmarshal(msg.Data, &order)
		if err != nil {
			fmt.Println("Error unmarshalling order: ", err)
			return
		}

		var response controllerResponse

		resp, err := nc.Request(fmt.Sprintf("%s.%s.%s", subjectStockManagerPrefix, order.BeanType, "get"), []byte(""), 2*time.Second)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		stockQuantity, err := strconv.Atoi(string(resp.Data))
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// If the quantity of coffee is greater than the amount of coffee needed for the order
		// then schedule the order
		// else return an error response
		log.Printf("🤔 Order ask for %d (size %s), we have currently %d", coffeeQuantityMap[order.Size], order.Size, stockQuantity)
		if coffeeQuantityMap[order.Size] <= stockQuantity {
			log.Println("✅ Order successfully submitted")
			response.Status = "success"
			response.Message = "The coffee has been successfully scheduled"
		} else {
			log.Println("❌ Not enough stock")
			response.Status = "error"
			response.Message = fmt.Sprintf("Insufficient coffee for type %s", order.BeanType)
		}

		// Notify coffee-makers that the order is scheduled

		if response.Status == "success" {
			err = nc.Publish(fmt.Sprintf("%s.%s", coffeeMakersSubjectPrefix, order.BeanType), msg.Data)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		jsonData, _ := json.Marshal(response)
		log.Printf("🗣️ Status: %s, Message: %s\n", response.Status, response.Message)
		msg.Respond(jsonData)
	})
	if err != nil {
		log.Fatal("connect to nats: ", err)
	}

	defer sub.Unsubscribe()

	log.Println("⌛ Waiting for orders...")
	// wait forever
	select {}

}
