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
/*
	log.Println(ft.InitTrackerForTransfer())
	log.Println("----")
	data, state := ft.GetTrackerInfo()
	log.Printf("Get: [ % #x ], state: %v \n", data, state)
	data, state = ft.RunDataBankOpcode('\x02')
	log.Printf("Get: [ % #x ], state: %v \n", data, state)
	data, state = ft.RunDataBankOpcode('\x00')
	log.Printf("Get: [ % #x ], state: %v \n", data, state)
	data, state = ft.RunDataBankOpcode('\x04')
	log.Printf("Get: [ % #x ], state: %v \n", data, state)
	data, state = ft.RunDataBankOpcode('\x02')
	log.Printf("Get: [ % #x ], state: %v \n", data, state)
	data, state = ft.RunDataBankOpcode('\x01')
	log.Printf("Get: [ % #x ], state: %v \n", data, state)
*/
	c := FitbitClient{
		FitbitBase: &ft,
	}
	log.Println(c.GetRemoteInfo())
	log.Println(c)
}
