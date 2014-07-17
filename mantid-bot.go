package main

import (
  "github.com/thoj/go-ircevent"
  "fmt"
  "regexp"
  "net/http"
  "io/ioutil"
  "code.google.com/p/go.net/html"
  "strings"
)

var roomName = "#mantid-talk"
var tracURL = "http://trac.mantidproject.org/mantid/ticket/"

var ticketNumberMatcher = regexp.MustCompile(`#\d{4}`)
var ticketTitleMatcher = regexp.MustCompile(`\((.*?)\)`)

var con = irc.IRC("mantid-bot", "mantid-bot")

func main() {
  //Connect to Freenode
  err := con.Connect("irc.freenode.net:6667");
  if err != nil {
    fmt.Println("Failed connecting");
    return;
  }

  //Join channel
  con.AddCallback("001", func (e *irc.Event) {
    con.Join(roomName);
  })

  //Handle new messages
  con.AddCallback("PRIVMSG", handleMessage);

  con.Loop()
}

func handleMessage(e *irc.Event) {
  //Try to extract a Trac ticket number from message
  ticketString := ticketNumberMatcher.FindString(e.Message());

  //Got a message with a Trac ticket
  if(ticketString != "") {
    ticketURL := tracURL + ticketString[1:]
    ticketTitle, ticketStatus := getTicketInfo(ticketURL);

    if ticketTitle != "" {
      con.Privmsg(roomName, fmt.Sprintf("%s: %s (%s)", ticketString, ticketTitle, ticketStatus))
      con.Privmsg(roomName, fmt.Sprintf("%s: %s", ticketString, ticketURL))
    } else {
      con.Privmsg(roomName, fmt.Sprintf(
        "There are over 9000 tickets, but %s is not one of them", ticketString))
    }
  }
}

func getTicketInfo(url string) (string, string) {
  r, err := http.Get(url)
  if(err != nil) {
    return "", ""
  }

  body, err := ioutil.ReadAll(r.Body)
  if(err != nil) {
    return "", ""
  }

  doc, err := html.Parse(strings.NewReader(string(body)))
  if err != nil {
    return "", ""
  }

  return htmlFindTitle(doc), htmlFindStatus(doc);
}

func htmlFindTitle(n *html.Node) string {
  if n.Type == html.ElementNode && n.Data == "title" {
    rawTitle := ticketTitleMatcher.FindString(n.FirstChild.Data)
    if rawTitle == "" {
      return ""
    }
    title := rawTitle[1:len(rawTitle)-1]

    return title
  }

  for c := n.FirstChild; c != nil; c = c.NextSibling {
    res := htmlFindTitle(c)
    if res != "" {
      return res;
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
        status := rawStatus[1:len(rawStatus)-1]

        return status
      }
    }
  }

  for c := n.FirstChild; c != nil; c = c.NextSibling {
    res := htmlFindStatus(c)
    if res != "" {
      return res;
    }
  }

  return ""
}
