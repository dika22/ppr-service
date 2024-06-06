package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
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
	Message    string `json:"message"`
	Image      string `json:"image"`
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

		err := validateRequest(req.PhoneNumber, req.Message)
		if  err != nil {
			c.JSON(400, gin.H{
				"message": "error send message",
			})
			return
		}

		fmt.Println("log image", setImage(req.Image))

		uploaded := whatsmeow.UploadResponse{}
		msg := &waProto.Message{}
		// upload image
		if req.Image != "" {
			client.Connect()
			// if err != nil {
			// 	return nil, err
			// }
			// resUploaded, err := uploadImage(client, req.Image)
			// if err != nil {
			// 	c.JSON(400, gin.H{
			// 		"message": "error upload file atau image",
			// 	})
			// 	return
			// }

			// fmt.Println("log uploadedd", uploaded)

			// uploaded = *resUploaded
			fileName := setImage(req.Image)
			data, err := os.ReadFile(fileName)
			if err != nil {
				log.Errorf("Failed to read %s: %v", fileName, err)
			}
			uploaded , err = client.Upload(context.Background(), data, whatsmeow.MediaImage)
			if err != nil {
				log.Errorf("Failed to upload file: %v", err)
			}

			msg = &waProto.Message{ImageMessage: &waProto.ImageMessage{
				Caption:       &req.Message,
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(http.DetectContentType(data)),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(data))),
			}}

		} else {
			msg = &waProto.Message{Conversation: proto.String(req.Message)}
		}

		fmt.Println("msg", msg)

		phoneNumber, _ := parseJID(req.PhoneNumber)
		resp, err := client.SendMessage(context.Background(), phoneNumber, msg, whatsmeow.SendRequestExtra{
			MediaHandle: uploaded.Handle,
		})

		if err != nil {
			err := client.Connect()
			if err != nil {
				fmt.Println("error connect client", err)
				c.JSON(400, gin.H{
					"message": err.Error(),
				})
			}

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

func validateRequest(phoneNumber, msg string) error {
	if phoneNumber == "" {
		return errors.New("phone Number not empty")
	}

	if msg == "" {
		return errors.New("phone Number not empty")
	}

	return nil
}

func uploadImage(client *whatsmeow.Client, fileBase64 string) (*whatsmeow.UploadResponse, error)  {
	err := client.Connect()
	if err != nil {
		return nil, err
	}

	fileName := setImage(fileBase64)
	data, err := os.ReadFile(fileName)
	if err != nil {
		log.Errorf("Failed to read %s: %v", fileName, err)
		return nil, err
	}
	uploaded , err := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		log.Errorf("Failed to upload file: %v", err)
	}

	return &uploaded, nil
}

func setImage(imageBase64 string) string {
	// Decode string Base64 menjadi byte array
	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		fmt.Println("Error decoding Base64 string:", err)
		return ""
	}

	// Nama file gambar yang akan disimpan
	fileName := "image_ppr.png"

	// Simpan byte array sebagai file gambar
	err = ioutil.WriteFile(fileName, imageData, 0644)
	if err != nil {
		fmt.Println("Error saving image:", err)
		return ""
	}

	fmt.Println("Image saved successfully as", fileName)

	return fileName
}