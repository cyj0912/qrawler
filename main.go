package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/html"
)

type crawlReq struct {
	url string
}

type pageContent struct {
	url  string
	body io.Reader
}

func crawlWorker(queue chan crawlReq) chan pageContent {
	resultQueue := make(chan pageContent)
	go func() {
		for front := range queue {
			fmt.Println("Sending GET request to", front.url)
			resp, err := http.Get(front.url)
			if err != nil {
				fmt.Println("Requested URL", front.url, "produced an error:", err)
				continue
			}

			bodyData, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Requested URL", front.url, "failed to read body:", err)
				continue
			}

			saveToDisk(front.url, bodyData)
			resultQueue <- pageContent{url: front.url, body: bytes.NewBuffer(bodyData)}
		}
		close(resultQueue)
	}()
	return resultQueue
}

type parsedPage struct {
	url       string
	text      []string
	neighbors []string
}

func parser(in <-chan pageContent) chan parsedPage {
	out := make(chan parsedPage)
	go func() {
		for pc := range in {
			doc, err := html.Parse(pc.body)
			if err != nil {
				log.Fatalln(pc.url, err)
				continue
			}
			// Parse url for host, so that relative links are processed correctly
			baseUrl, err := url.Parse(pc.url)
			if err != nil {
				fmt.Println("URL is malformed, page causing it is", pc.url)
				continue
			}

			// Recursively scan the html document for links
			parsedPage := parsedPage{url: pc.url}
			parsedPage.neighbors = make([]string, 0)

			var nodeProc func(node *html.Node)
			nodeProc = func(node *html.Node) {
				if node.Type == html.ElementNode && node.Data == "a" {
					// Is a link
					for _, attr := range node.Attr {
						if attr.Key == "href" {
							neighborUrl, err := cleanUrl(baseUrl, attr.Val)
							if err != nil {
								continue
							}
							parsedPage.neighbors = append(parsedPage.neighbors, neighborUrl)
						}
					}
				} else if node.Type == html.TextNode {
					parsedPage.text = append(parsedPage.text, node.Data)
				}

				for c := node.FirstChild; c != nil; c = c.NextSibling {
					nodeProc(c)
				}
			}
			nodeProc(doc)
			out <- parsedPage
		}
		close(out)
	}()
	return out
}

type controlStates struct {
	WaitingQueue []string
	QueuedSet    map[string]bool
}

func main() {
	queue := make(chan crawlReq, 4)

	crawledQueue := crawlWorker(queue)
	parsed := parser(crawledQueue)

	// Output
	outputFile, err := os.OpenFile("qrawler.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("Failed to open log file", err)
	}

	// Main control states
	states := controlStates{
		WaitingQueue: make([]string, 0),
		QueuedSet:    make(map[string]bool)}

	// Serialization
	loadStates := func() bool {
		file, err := os.Open("qrawler_states.json")
		if err != nil {
			fmt.Println(err)
			return false
		}
		buf, err := io.ReadAll(file)
		if err != nil {
			fmt.Println(err)
			return false
		}
		err = json.Unmarshal(buf, &states)
		if err != nil {
			fmt.Println(err)
			log.Fatalln(err)
		}
		return true
	}

	saveStates := func() {
		j, err := json.Marshal(states)
		if err != nil {
			log.Fatalln(err)
		}
		file, err := os.OpenFile("qrawler_states.json", os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalln(err)
		}
		_, err = file.Write(j)
		if err != nil {
			log.Fatalln(err)
		}
	}

	enqueue := func(result parsedPage) {
		for _, url := range result.neighbors {
			_, isInSet := states.QueuedSet[url]
			if !isInSet {
				states.WaitingQueue = append(states.WaitingQueue, url)
				states.QueuedSet[url] = true
			}
		}

		// Write to disk as well
		fmt.Fprintln(outputFile, result)
	}

	dequeue := func() string {
		elem := states.WaitingQueue[0]
		states.WaitingQueue = states.WaitingQueue[1:]
		return elem
	}

	// Setup signal handler
	osChan := make(chan os.Signal, 1)
	signal.Notify(osChan, os.Interrupt, syscall.SIGTERM)

	// Kickoff
	if !loadStates() {
		srcUrl := "https://en.wikipedia.org/wiki/Keebler_Company"
		fmt.Println("Starting anew from source url:", srcUrl)
		queue <- crawlReq{url: srcUrl}
	}

	for {
		if len(states.WaitingQueue) != 0 {
			select {
			case result := <-parsed:
				enqueue(result)
			case queue <- crawlReq{url: dequeue()}:
			case <-osChan:
				saveStates()
				fmt.Println("States successfully saved, exitting...")
				os.Exit(0)
			}
		} else {
			enqueue(<-parsed)
		}
	}
}
