package main

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

func ClientConnect(a smtp.Auth, hostname string, port string) (client *smtp.Client, err error) {
	switch port {
	case "465":
		if conn, e := tls.Dial("tcp", fmt.Sprintf("%s:%s", hostname, port), nil); e != nil {
			err = e
		} else if c, e := smtp.NewClient(conn, hostname); e != nil {
			err = e
		} else if e := c.Auth(a); e != nil {
			err = e
		} else {
			client = c
		}
	case "587":
		if c, e := smtp.Dial(fmt.Sprintf("%s:%s", hostname, port)); e != nil {
			err = e
		} else if e := c.StartTLS(&tls.Config{ServerName: hostname}); e != nil {
			err = e
		} else if e := c.Auth(a); e != nil {
			err = e
		} else {
			client = c
		}
	default:
		if c, e := smtp.Dial(fmt.Sprintf("%s:%s", hostname, port)); e != nil {
			err = e
		} else if e := c.Auth(a); e != nil {
			err = e
		} else {
			client = c
		}
	}
	return
}
