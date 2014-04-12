package main

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	CLIENTUUID = "2ea32002-a079-48f4-8020-0badd22939e3"
	FITBITHOST = "https://client.fitbit.com"
	STARTPATH  = "/device/tracker/uploadData"
)

type FitbitConfig struct {
	ResponseInfo Response   `xml:"response"`
	RemoteOps    []RemoteOp `xml:"device>remoteOps>remoteOp"`
}

type FitbitClient struct {
	*FitbitBase
}

type Response struct {
	Body string `xml:",chardata"`
	Host string `xml:"host,attr"`
	Path string `xml:"path,attr"`
}

type RemoteOp struct {
	OpCode      string `xml:"opCode"`
	PayloadData string `xml:"payloadData"`
}

func (c *FitbitClient) UploadData() error {
	//init_tracker_for_transfer
	v := url.Values{}
	weburl := FITBITHOST + STARTPATH
	err := c.InitTrackerForTransfer()
	log.Println("end init----------")
	if err != nil {
		return err
	}
	defer c.CommandSleep()
	client := http.Client{}
	v.Set("beaconType", "standard")
	v.Set("clientMode", "standard")
	v.Set("clientVersion", "1.0")
	v.Set("os", "fitbitd")
	v.Set("clientId", CLIENTUUID)
	for {
		log.Println(weburl, v)
		resp, err := client.PostForm(weburl, v)
		if err != nil {
			return err
		}
		body, err := ioutil.ReadAll(resp.Body)
		config := FitbitConfig{}
		if err == nil {
			err = xml.Unmarshal(body, &config)
		}
		resp.Body.Close()
		log.Println(string(body))
		v, err = url.ParseQuery(config.ResponseInfo.Body)
		if err != nil {
			return err
		}
		for i, op := range config.RemoteOps {
			opcode, err := base64.StdEncoding.DecodeString(op.OpCode)
			payload, err := base64.StdEncoding.DecodeString(op.PayloadData)
			code, err := c.RunOpcode(opcode, payload)
			if err != nil {
				return err
			}
			resp := base64.StdEncoding.EncodeToString(code)
			log.Printf("opCode[%d]: %s, payload: %s, response: %s\n", i, op.OpCode, op.PayloadData, resp)
			v.Set(fmt.Sprintf("opResponse[%d]", i), resp)
			v.Set(fmt.Sprintf("opStatus[%d]", i), "success")
		}
		if config.ResponseInfo.Host == "" {
			break
		}
		v.Set("beaconType", "standard")
		v.Set("clientMode", "standard")
		v.Set("clientVersion", "1.0")
		v.Set("os", "fitbitd")
		v.Set("clientId", CLIENTUUID)
		weburl = "http://" + config.ResponseInfo.Host + config.ResponseInfo.Path
	}
	return err
}

type SyncTask struct {
	exitChannel chan int
}

func (s *SyncTask) Run() {
	ticker := time.Tick(time.Second * 600)
	for {
		fb := FitbitBase{}
		err := fb.Open()
		if err == nil {
			err = fb.SettingUp()
			if err == nil {
				c := FitbitClient{
					FitbitBase: &fb,
				}
				log.Println("start sync")
				err = c.UploadData()
				if err != nil {
					log.Println("sync failed")
				} else {
					log.Println("sync success")
				}
			}
			fb.Close()
		}
		select {
		case <-ticker:
			continue
		case <-s.exitChannel:
			return
		}
	}
}
func (s *SyncTask) Stop() {
	close(s.exitChannel)
}
