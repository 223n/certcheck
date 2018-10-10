package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"time"

	yaml "gopkg.in/yaml.v2"
)

var (
	// Version - certcheck version
	Version = "1.2.2"
	// Revision - revision version
	Revision = "6"
)

// Data - Setting File Struct
type Data struct {
	Targets []Target `yaml:"targets"`
	Slack   Slack    `yaml:"slack"`
}

// Target - Cert Check Target Struct
type Target struct {
	Name      string `yaml:"name"`
	Endpoint  string `yaml:"endpoint"`
	Threshold int    `yaml:"threshold"`
	URL       string `yaml:"hook_url"`
	Channel   string `yaml:"channel"`
	Username  string `yaml:"username"`
	Icon      string `yaml:"icon"`
}

// Slack - Notify Setting Struct
type Slack struct {
	URL      string `yaml:"hook_url"`
	Channel  string `yaml:"channel"`
	Username string `yaml:"username"`
	Icon     string `yaml:"icon"`
}

// SlackJSON - Slack Properties
type SlackJSON struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
	Text     string `json:"text"`
	Icon     string `json:"icon_emoji"`
}

func main() {
	filename := flag.String("c", "certcheck.yml", "config file name")
	isVersion1 := flag.Bool("v", false, "prints current version")
	isVersion2 := flag.Bool("version", false, "prints current version")
	flag.Parse()

	if *isVersion1 || *isVersion2 {
		log.Printf(getVersion())
	} else {
		buf, err := ioutil.ReadFile(*filename)
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
					postSlack(d.Slack, tgt, message)
				}
				log.Printf(message)
			}
		}
	}
}

func getVersion() string {
	return fmt.Sprintf("%s %s/%s, build %s", Version, runtime.GOOS, runtime.GOARCH, Revision)
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
	if tgt.Threshold < 0 {
		log.Printf("threshold is less than 0. (key: %d): tgt.Endpoint", key)
		result = false
	}
	return result
}

/**
 *  Portions are:
 *    Copyright (C) 2017 ynozue (https://github.com/ynozue)
 *    Originally under Apache License Version 2.0, http://www.apache.org/licenses/LICENSE-2.0
 **/
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
			since := int(expireJSTTime.Sub(time.Now()).Hours() / 24)
			if !th.Before(expireJSTTime) {
				message = fmt.Sprintf("Cert Warning: %s expire: %s at %d days", endpoint, expire, since)
			} else {
				message = fmt.Sprintf("Cert OK: %s expire: %s at %d days", endpoint, expire, since)
				result = false
			}
		}
	}
	return message, result
}

func postSlack(slack Slack, tgt Target, message string) bool {
	slackProperties := SlackJSON{}
	// Channel
	if slack.Channel != "" {
		slackProperties.Channel = slack.Channel
	}
	if tgt.Channel != "" {
		slackProperties.Channel = tgt.Channel
	}
	// Username
	if slack.Username != "" {
		slackProperties.Username = slack.Username
	}
	if tgt.Username != "" {
		slackProperties.Username = tgt.Username
	}
	// Text
	slackProperties.Text = message
	// Icon
	if slack.Icon != "" {
		slackProperties.Icon = slack.Icon
	}
	if tgt.Icon != "" {
		slackProperties.Icon = tgt.Icon
	}
	params, _ := json.Marshal(slackProperties)
	var slackURL = slack.URL
	if tgt.URL != "" {
		slackURL = tgt.URL
	}
	resp, err := http.PostForm(
		slackURL,
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
	return true
}
