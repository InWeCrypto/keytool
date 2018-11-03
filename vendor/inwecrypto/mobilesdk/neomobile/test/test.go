package main

import (
	"encoding/json"
	"fmt"
)

type Server struct {
	ServerName string
	ServerIP   string
}

type Serverslice struct {
	Servers []Server
}

func main() {
	var s []string
	str := `["serverName","上海","serverIP","127.0.0.1"]`
	json.Unmarshal([]byte(str), &s)
	fmt.Println(s)
}
