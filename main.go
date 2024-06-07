package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	route "ppr-service/route"
	svc "ppr-service/service"
)

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	// init .env
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error getting env", err)
	}

	dbConfig := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=disable", os.Getenv("USER_DB"), os.Getenv("PASSWORD_DB"), os.Getenv("HOST_DB"), os.Getenv("NAME_DB"))
	container, err := sqlstore.New("postgres", dbConfig, dbLog)
	if err != nil {
		panic(err)
	}

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	// set gin
	r := gin.Default()
	r.Use(gin.Recovery())

	ps := svc.NewPprService(client)
	route.RouterGroup(r, ps)

	if err := r.Run(fmt.Sprintf(":%s", os.Getenv("HTTP_PORT"))); err != nil {
		fmt.Println("error", err)
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	}
}
