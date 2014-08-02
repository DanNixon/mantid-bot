package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"encoding/json"
	"fmt"
	"github.com/thoj/go-ircevent"
	"log"
	"net/http"
	"regexp"
)

var (
	server   = "irc.freenode.net:6667"
	roomName = "#mantid-talk"

	tracURL    = "http://trac.mantidproject.org/mantid/ticket/"
	jenkinsAPI = "http://builds.mantidproject.org/api/json"

	jenkinsStatus = map[string]string{
		"red":          "failed",
		"red_anime":    "failed (in progress)",
		"yellow":       "built with warnings",
		"yellow_anime": "built with warnings (in progress)",
		"blue":         "passed",
		"blue_anime":   "passed (in progress)",
	}

	ticketNumberMatcher = regexp.MustCompile(`#\d{4,5}`)
	ticketTitleMatcher  = regexp.MustCompile(`\((.*?)\)`)
	buildJobMatcher     = regexp.MustCompile(`!(.+?)\b`)
	allJobsQueryMatcher = regexp.MustCompile(`!builds\b`)

	con = irc.IRC("mantid-bot", "mantid-bot")
)

type Job struct {
	Name  string `json:"name"`
	Url   string `json:"url"`
	Color string `json:"color"`
}

type BuildServer struct {
	NodeDescription string `json:"nodeDescription"`
	Jobs            []Job  `json:"jobs"`
}

func main() {
	log.Printf("Connecting to %s\n", server)

	err := con.Connect(server)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Joining %s\n", roomName)
	con.AddCallback("001", func(e *irc.Event) {
		con.Join(roomName)
	})

	//Handle new messages
	con.AddCallback("PRIVMSG", handleMessage)

	con.Loop()
}

func handleMessage(e *irc.Event) {
	//Try to extract a Trac ticket number from message
	ticketString := ticketNumberMatcher.FindString(e.Message())

	//Try to extract a build job
	buildJob := buildJobMatcher.FindString(e.Message())

	//Got a message with a Trac ticket
	if ticketString != "" {
		ticketURL := tracURL + ticketString[1:]
		ticketTitle, ticketStatus := getTicketInfo(ticketURL)

		if ticketTitle != "" {
			con.Privmsg(roomName, fmt.Sprintf("%s: %s (%s)", ticketString, ticketTitle, ticketStatus))
			con.Privmsg(roomName, fmt.Sprintf("%s: %s", ticketString, ticketURL))
		} else {
			con.Privmsg(roomName, fmt.Sprintf(
				"There are over 9000 tickets, but %s is not one of them", ticketString))
		}
	}

	if allJobsQueryMatcher.MatchString(e.Message()) {
		jobs := getAllBuildJobs()
		var jobsBuffer bytes.Buffer

		for _, job := range jobs {
			jobsBuffer.WriteString(job.Name)
			jobsBuffer.WriteString(" ")
		}

		con.Privmsg(roomName, fmt.Sprintf("All build server jobs: %s", jobsBuffer.String()))
		return
	}

	if buildJob != "" {
		jobName := buildJob[1:]
		jobResult := getBuildStatus(jobName)

		if jobResult != "" {
			con.Privmsg(roomName, fmt.Sprintf("Build job %s has %s", jobName, jobResult))
		} else {
			con.Privmsg(roomName, fmt.Sprintf("This is not the build job you are looking for (%s)", jobName))
		}
	}
}

func getTicketInfo(url string) (string, string) {
	r, err := http.Get(url)
	if err != nil {
		return "", ""
	}

	doc, err := html.Parse(r.Body)
	if err != nil {
		return "", ""
	}

	return htmlFindTitle(doc), htmlFindStatus(doc)
}

func htmlFindTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		rawTitle := ticketTitleMatcher.FindString(n.FirstChild.Data)
		if rawTitle == "" {
			return ""
		}
		title := rawTitle[1 : len(rawTitle)-1]

		return title
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res := htmlFindTitle(c)
		if res != "" {
			return res
		}
	}

	return ""
}

func htmlFindStatus(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "span" {
		for _, a := range n.Attr {
			if a.Key == "class" && a.Val == "status" {
				rawStatus := ticketTitleMatcher.FindString(n.FirstChild.Data)
				if rawStatus == "" {
					return ""
				}
				status := rawStatus[1 : len(rawStatus)-1]

				return status
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res := htmlFindStatus(c)
		if res != "" {
			return res
		}
	}

	return ""
}

func getBuildStatus(build string) string {
	jobs := getAllBuildJobs()

	for _, job := range jobs {
		if job.Name == build {
			return jenkinsStatus[job.Color]
		}
	}

	return ""
}

func getAllBuildJobs() []Job {
	log.Printf("Getting all Jenkins jobs from ", jenkinsAPI)

	r, err := http.Get(jenkinsAPI)
	if err != nil {
		return nil
	}

	var res BuildServer
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&res)

	return res.Jobs
}
