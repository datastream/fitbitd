package main

import (
	"log"
)

func main() {
	ft := FitbitBase{}
	err := ft.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer ft.Close()
	log.Printf("Connecting to endpoint...")
	log.Printf("- %#v", ft.dev.Descriptor)
	err = ft.SettingUp()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(ft.base.Reset())
}
