package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"ppr-service/models"
	"ppr-service/utils"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

type PprService interface {
	Login(ctx context.Context) ([]byte, error)
	SendMessage(req models.Message) (*whatsmeow.SendResponse, error)
}

type pprService struct{
	client *whatsmeow.Client
}

func NewPprService(cli *whatsmeow.Client) PprService  {
	return &pprService{
		client: cli,
	}
}  

func (s pprService) Login(ctx context.Context) ([]byte, error) {
	qrString := GenerateQRCode(ctx, s.client)
	if qrString == nil {
		return nil, errors.New("already login")
	}
	qrCode := utils.GetQrCode(*qrString)
	return qrCode, nil	
}


func (s pprService) SendMessage(req models.Message) (*whatsmeow.SendResponse, error) {
	upload := &whatsmeow.UploadResponse{}
	msg := &waProto.Message{}
	if req.Image != "" {
		fileName := setImage(req.Image)
		data, err := os.ReadFile(fileName)
		if err != nil {
			return nil, err
		}
		// upload image
		uploaded, err := uploadImage(s.client, data)
		if err != nil {
			return nil, err
		}

		upload = uploaded
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

	phoneNumber, _ := utils.ParseJID(req.PhoneNumber)
	resp, err := s.client.SendMessage(context.Background(), phoneNumber, msg, whatsmeow.SendRequestExtra{
		MediaHandle: upload.Handle,
	})

	if err != nil {
		setClientConnection(s.client)
	}

	return &resp, nil
}

func GenerateQRCode(ctx context.Context, client *whatsmeow.Client) *string {
	// set and check connection
	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(ctx)
		setClientConnection(client)
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

func setClientConnection(client *whatsmeow.Client)  {
	if !client.IsConnected() {
		err := client.Connect()
		fmt.Println("error client connection", err)
	}
}

func uploadImage(client *whatsmeow.Client, data []byte) (*whatsmeow.UploadResponse, error)  {
	// set and check connection
	setClientConnection(client)
	uploaded , err := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
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
