package utils

import (
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow/types"
)

func GetQrCode(qrString string) []byte  {
	var png []byte
  	png, err := qrcode.Encode(qrString, qrcode.Medium, 256)

	if err != nil {
		fmt.Println("error", err)
	}

  	return png
}

func ParseJID(arg string) (types.JID, bool) {
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), true
	} else {
		recipient, err := types.ParseJID(arg)
		if err != nil {
			fmt.Printf("Invalid JID %s: %v", arg, err)
			return recipient, false
		} else if recipient.User == "" {
			fmt.Printf("Invalid JID %s: no server specified", arg)
			return recipient, false
		}
		return recipient, true
	}
}