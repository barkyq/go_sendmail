package main

import (
	"encoding/json"
	"io"
	"net/smtp"
	"strings"
)

// LoadConfig loads a configuration file (json encoded) and returns the relevant information.
// addr (hostname:port format) is the remote address for which to make a connection.
// folder_list (map[local_name]remote_name) is the list of folders for which to sync; default values are "inbox", "sent", and "archive".
// directory is the root directory containing the maildir
// mem represents the local representation of the mailbox

type UserConfig struct {
	SMTPServer   string `json:"smtp_server"`
	IMAPServer   string `json:"imap_server"`
	Type         string `json:"type"`
	User         string `json:"user"`
	Password     string `json:"password"`
	ClientID     string `json:"clientid"`
	ClientSecret string `json:"clientsecret"`
	RefreshToken string `json:"refreshtoken"`
}

func LoadConfig(r io.Reader) (smtp_hostname string, smtp_port string, smtp_a smtp.Auth, e error) {
	user_config := new(UserConfig)

	// load config from os.Stdin
	dec := json.NewDecoder(r)
	if e = dec.Decode(user_config); e != nil {
		return
	}
	// directory = userinfo["directory"]
	// os.MkdirAll(directory, os.ModePerm)

	smtp_hostname = strings.Split(user_config.SMTPServer, ":")[0]
	smtp_port = strings.Split(user_config.SMTPServer, ":")[1]
	switch user_config.Type {
	case "plain":
		smtp_a = smtp.PlainAuth("", user_config.User, user_config.Password, smtp_hostname)
	case "gmail":
		// see https://developers.google.com/gmail/imap/imap-smtp#session_length_limits
		config, token := Gmail_Generate_Token(user_config.ClientID, user_config.ClientSecret, user_config.RefreshToken)
		smtp_a = XOAuth2(user_config.User, config, token)
	case "outlook":
		config, token := Outlook_Generate_Token(user_config.ClientID, user_config.RefreshToken)
		smtp_a = XOAuth2(user_config.User, config, token)
	}
	return
}
