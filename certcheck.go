package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// Data - Setting File Struct
type Data struct {
	Targets []Target `yaml:"targets"`
	Slacks  []Slack  `yaml:"slacks"`
}

// Target - Cert Check Target Struct
type Target struct {
	Name      string `yaml:"name"`
	Endpoint  string `yaml:"endpoint"`
	SlackNo   int    `yaml:"slackno"`
	Threshold int    `yaml:"threshold"`
}

// Slack - Notify Setting Struct
type Slack struct {
	No       int    `yaml:"no"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
}

// SlackJSON - Slack Properties
type SlackJSON struct {
	Text     string `json:"text"`
	Username string `json:"username"`
}

func main() {
	buf, err := ioutil.ReadFile("certcheck.yml")
	if err != nil {
		log.Fatal(err)
		return
	}
	var d Data
	err = yaml.Unmarshal(buf, &d)
	if err != nil {
		log.Fatal(err)
		return
	}
	for key, tgt := range d.Targets {
		if checkParam(key, tgt) {
			message, result := getAPI(tgt.Endpoint, tgt.Threshold)
			if result {
				postSlack(d.Slacks, message, tgt.SlackNo)
			}
			log.Printf(message)
		}
	}
}

func checkParam(key int, tgt Target) bool {
	var result = true
	if tgt.Name == "" {
		log.Printf("name is empty. (key: %d): tgt.Endpoint", key)
		result = false
	}
	if tgt.Endpoint == "" {
		log.Printf("endpoint is empty. (key: %d): tgt.Endpoint", key)
		result = false
	}
	if tgt.SlackNo < 0 {
		log.Printf("slackno is less than 0. (key: %d): tgt.Endpoint", key)
		result = false
	}
	if tgt.Threshold < 0 {
		log.Printf("threshold is less than 0. (key: %d): tgt.Endpoint", key)
		result = false
	}
	return result
}

func getAPI(endpoint string, threshold int) (string, bool) {
	var message = ""
	var result = true
	resp, err := http.Get(endpoint)
	if err != nil {
		message = fmt.Sprintf("NG: %s", err)
	} else {
		defer resp.Body.Close()
		expire := "-"
		if len(resp.TLS.PeerCertificates) > 0 {
			expireUTCTime := resp.TLS.PeerCertificates[0].NotAfter
			expireJSTTime := expireUTCTime.In(time.FixedZone("Asia/Tokyo", 9*60*60))
			expire = expireJSTTime.Format("2006/01/02 15:04")
			th := time.Now().AddDate(0, 0, threshold)
			if !th.Before(expireJSTTime) {
				message = fmt.Sprintf("Warning (expire=%s): %s", expire, endpoint)
			} else {
				message = fmt.Sprintf("OK (expire=%s): %s", expire, endpoint)
				result = false
			}
		}
	}
	return message, result
}

func postSlack(slacks []Slack, message string, slackNo int) bool {
	for _, slack := range slacks {
		if slack.No == slackNo {
			if slack.URL == "" {
				log.Printf("slack URL is empty. (key: %d)", slackNo)
				return false
			} else if slack.Username == "" {
				log.Printf("slack Username is empty. (key: %d)", slackNo)
				return false
			}
			params, _ := json.Marshal(SlackJSON{
				message,
				slack.Username})
			resp, err := http.PostForm(
				slack.URL,
				url.Values{"payload": {string(params)}},
			)
			if err != nil {
				log.Fatal(err)
				return false
			}
			defer resp.Body.Close()
			_, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
				return false
			}
		}
	}
	return true
}
