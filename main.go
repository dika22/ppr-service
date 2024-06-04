package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"context"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/skip2/go-qrcode"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

type Message struct {
	PhoneNumber string `json:"phoneNumber"`
	Message string `json:"message"`
}

var  log waLog.Logger

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("postgres", "postgresql://adhika:adhika@127.0.0.1/postgres?sslmode=disable", dbLog)
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

	// init .env
	err = godotenv.Load()
	if err != nil {
		log.Errorf("Error getting env, %v", err)
	}

	// set gin
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// end point /login
	r.GET("/login", func(c *gin.Context) {
		qrString := GenerateQRCode(client)
		if qrString == nil {
			c.JSON(400, gin.H{
				"message": "already login",
			})
		}
		qrCode := getQrCode(*qrString)
		c.Data(http.StatusOK, "image/png", qrCode)
	})

	// endpoint send-message
	r.POST("/send-message", func(c *gin.Context) {
		req := Message{}
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		msg := &waProto.Message{Conversation: proto.String(req.Message)}
		phoneNumber, _ := parseJID(req.PhoneNumber)
		resp, err := client.SendMessage(context.Background(), phoneNumber, msg)
		if err != nil {
			fmt.Println("error", err)
			c.JSON(400, gin.H{
				"message": "error send message",
			})
			return
		} else {
			c.JSON(200, gin.H{
				"message": "success send message",
				"data" : resp,
			})
			return
		}
	})

	if err := r.Run(fmt.Sprintf(":%s", os.Getenv("HTTP_PORT"))); err != nil {
		log.Errorf("error", err)
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}


func getQrCode(qrString string) []byte  {
	var png []byte
  	png, err := qrcode.Encode(qrString, qrcode.Medium, 256)

	if err != nil {
		fmt.Println("error", err)
	}

  	return png
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	}
}

func GenerateQRCode(client *whatsmeow.Client) *string {
	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err := client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				return &evt.Code
			} else {
				return &evt.Event
			}
		}
	} else {
		// Already logged in, just connect
		err := client.Connect()
		if err != nil {
			return nil
		}
	}

	return nil
}


func parseJID(arg string) (types.JID, bool) {
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), true
	} else {
		recipient, err := types.ParseJID(arg)
		if err != nil {
			log.Errorf("Invalid JID %s: %v", arg, err)
			return recipient, false
		} else if recipient.User == "" {
			log.Errorf("Invalid JID %s: no server specified", arg)
			return recipient, false
		}
		return recipient, true
	}
}