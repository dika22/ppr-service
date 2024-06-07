package models

import "errors"

type Message struct {
	PhoneNumber string `json:"phoneNumber"`
	Message     string `json:"message"`
	Image       string `json:"image"`
}


func ValidateRequest(phoneNumber, msg string) error {
	if phoneNumber == "" {
		return errors.New("phone Number not empty")
	}

	if msg == "" {
		return errors.New("phone Number not empty")
	}

	return nil
}