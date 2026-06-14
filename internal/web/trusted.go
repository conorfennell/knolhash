package web

import (
	"log/slog"
	"net/http"
	"strings"
)

type TrustedSource struct {
	Label string
	URL   string
	Type  string
}

type TrustedTopic struct {
	Topic   string
	Sources []TrustedSource
}

type TrustedPerson struct {
	Handle string
	Name   string
	Topics []TrustedTopic
}

var trustedPeople = map[string]TrustedPerson{
	"cfennell": {
		Handle: "cfennell",
		Name:   "Conor Fennell",
		Topics: []TrustedTopic{
			{
				Topic: "Personal Finance",
				Sources: []TrustedSource{
					{Label: "r/irishpersonalfinance", URL: "https://www.reddit.com/r/irishpersonalfinance/", Type: "forum"},
				},
			},
			{
				Topic: "Clean Energy",
				Sources: []TrustedSource{
					{Label: "Just Have a Think", URL: "https://www.youtube.com/@JustHaveaThink", Type: "youtube-channel"},
					{Label: "BloombergNEF", URL: "https://about.bnef.com/", Type: "research"},
				},
			},
			{
				Topic: "News",
				Sources: []TrustedSource{
					{Label: "The Daily", URL: "https://www.nytimes.com/column/the-daily", Type: "podcast"},
					{Label: "Irish Times", URL: "https://www.irishtimes.com/", Type: "newspaper"},
					{Label: "The Guardian", URL: "https://www.theguardian.com/", Type: "newspaper"},
					{Label: "In the News (Irish Times)", URL: "https://www.irishtimes.com/podcasts/in-the-news/", Type: "podcast"},
				},
			},
			{
				Topic: "TV Reviews",
				Sources: []TrustedSource{
					{Label: "TV on the Radio", URL: "https://www.newstalk.com/podcasts/tv-on-the-radio", Type: "podcast"},
				},
			},
			{
				Topic: "Sports",
				Sources: []TrustedSource{
					{Label: "Second Captains", URL: "https://www.secondcaptains.com/", Type: "podcast"},
					{Label: "ACFC", URL: "https://www.youtube.com/@ACFC", Type: "youtube-channel"},
					{Label: "Squidge Rugby", URL: "https://www.youtube.com/@SquidgeRugby", Type: "youtube-channel"},
				},
			},
			{
				Topic: "Technology",
				Sources: []TrustedSource{
					{Label: "Hacker News", URL: "https://news.ycombinator.com/", Type: "news-aggregator"},
				},
			},
			{
				Topic: "Ukraine Russia War",
				Sources: []TrustedSource{
					{Label: "Ukraine: The Latest", URL: "https://www.youtube.com/@UkraineTheLatest", Type: "youtube-channel"},
				},
			},
		},
	},
}

type trustedPageData struct {
	TrustedPerson
	TotalSources int
	TotalTopics  int
}

func (s *Server) handleGetTrusted() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handle := strings.TrimPrefix(r.URL.Path, "/trusted/")
		handle = strings.Trim(handle, "/")

		if handle == "" {
			http.NotFound(w, r)
			return
		}

		person, ok := trustedPeople[handle]
		if !ok {
			http.NotFound(w, r)
			return
		}

		total := 0
		for _, t := range person.Topics {
			total += len(t.Sources)
		}

		data := trustedPageData{person, total, len(person.Topics)}
		if err := s.templates.ExecuteTemplate(w, "trusted", data); err != nil {
			slog.Error("trusted: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
