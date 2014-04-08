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

type FitbitClient struct {
	*FitbitBase  `xml:"-"`
	ResponseInfo Response   `xml:"response"`
	RemoteOps    []RemoteOp `xml:"device>remoteOps>remoteOp"`
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
	ok, err := c.InitTrackerForTransfer()
	log.Println("end init----------")
	if !ok {
		return err
	}
	defer c.CommandSleep()
	client := http.Client{}
	for {
		v.Set("beaconType", "standard")
		v.Set("clientMode", "standard")
		v.Set("clientVersion", "1.0")
		v.Set("os", "libfitbit")
		v.Set("clientId", CLIENTUUID)
		log.Println(weburl, v)
		resp, err := client.PostForm(weburl, v)
		c.ResponseInfo = Response{}
		c.RemoteOps = c.RemoteOps[:0]
		if err != nil {
			log.Println(err)
			break
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			err = xml.Unmarshal(body, c)
		}
		resp.Body.Close()
		log.Println(string(body))
		v, err = url.ParseQuery(c.ResponseInfo.Body)
		if err != nil {
			return err
		}
		for i, op := range c.RemoteOps {
			log.Printf("opCode[%d]: %s, payload: %s\n", i, op.OpCode, op.PayloadData)
			opcode, err := base64.StdEncoding.DecodeString(op.OpCode)
			payload, err := base64.StdEncoding.DecodeString(op.PayloadData)
			code, err := c.RunOpcode(opcode, payload)
			if err != nil {
				log.Println(err)
			}
			v.Set(fmt.Sprintf("opResponse[%d]", i), base64.StdEncoding.EncodeToString(code))
			v.Set(fmt.Sprintf("opStatus[%d]", i), "success")
		}
		if c.ResponseInfo.Host == "" {
			break
		}
		weburl = "http://" + c.ResponseInfo.Host + c.ResponseInfo.Path
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
					log.Println("sync failed", err)
				} else {
					log.Println("sync end")
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
