package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
)

func saveToDisk(urlString string, bodyData []byte) {
	u, err := url.Parse(urlString)
	if err != nil {
		fmt.Println("Failed to parse url to save to disk:", urlString)
	}

	rootDir := "content/" + u.Hostname()
	err = os.MkdirAll(rootDir, 0755)
	if err != nil {
		log.Fatalln(err)
	}

	if strings.HasSuffix(u.Path, "/") {
		u.Path = u.Path + "__default"
	}

	iLastSlash := strings.LastIndex(u.Path, "/")
	err = os.MkdirAll(rootDir+u.Path[0:iLastSlash], 0755)
	if err != nil {
		log.Fatalln(err)
	}

	file, err := os.OpenFile(rootDir+u.Path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	file.Write(bodyData)
	file.Close()
}
