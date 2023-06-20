package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	baseUrl         = "https://api.github.com/"
	apiPath         = "repos/<username>/<repo_name>/pulls?state=closed&sort=updated&direction=desc&per_page=100"
	timeMonthLayout = "January 2006"
)

var ghtoken string

type User struct {
	Login string `json:"login"`
}

type PullRequest struct {
	User            User   `json:"user"`
	MergedAt        string `json:"merged_at"`
	MergeCommitHash string `json:"merge_commit_sha"`
	MergeCommit     Commit `json:"merge_commit"`
	Title           string `json:"title"`
}

type Commit struct {
	Author Author `json:"author"`
}

type Author struct {
	Login string `json:"login"`
}

type SortedGroupedMergedByMonth struct {
	Month string
	Pulls []PullRequest
}

type ByMonth []SortedGroupedMergedByMonth

func (b ByMonth) Len() int {
	return len(b)
}
func (b ByMonth) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
func (b ByMonth) Less(i, j int) bool {
	date1, _ := time.Parse(timeMonthLayout, b[i].Month)
	date2, _ := time.Parse(timeMonthLayout, b[j].Month)
	return date1.Before(date2)
}

func main() {
	// Get Github username and repository name from command line arguments
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <username> <repo_name>")
		os.Exit(1)
	}
	userName := os.Args[1]
	repoName := os.Args[2]

	pulls := []PullRequest{}
	// Will fetch 4 pages
	for page := 1; page <= 5; page++ {
		err := fetchPulls(userName, repoName, &pulls, page)
		if err != nil {
			fmt.Printf("Error fetching pulls %v", err)
			os.Exit(1)
		}
	}

	// Group pull requests by month and print
	mergedByMonth := make(map[string][]PullRequest)
	for _, pull := range pulls {
		if pull.MergedAt == "" {
			continue
		}
		mergedAt, err := time.Parse(time.RFC3339, pull.MergedAt)
		if err != nil {
			fmt.Printf("Error parsing date: %s", err.Error())
			continue
		}
		if mergedAt.Before(time.Now().AddDate(-2, 0, 0)) {
			// Only consider PRs merged in the last year
			continue
		}

		month := mergedAt.Format(timeMonthLayout)
		pull.MergeCommit.Author.Login = getMergeCommitAuthor(pull.MergeCommitHash)
		mergedByMonth[month] = append(mergedByMonth[month], pull)
	}

	sortedGroupedMergedByMonth := make([]SortedGroupedMergedByMonth, 0)
	for month, groupedPulls := range mergedByMonth {
		sortedGroupedMergedByMonth = append(sortedGroupedMergedByMonth, SortedGroupedMergedByMonth{Month: month, Pulls: groupedPulls})
	}

	sort.Sort(ByMonth(sortedGroupedMergedByMonth))

	for _, groupedMergedByMonth := range sortedGroupedMergedByMonth {
		month := groupedMergedByMonth.Month
		pulls := groupedMergedByMonth.Pulls
		fmt.Printf("%s:\n", month)
		for _, pull := range pulls {
			fmt.Printf("%s - Author: %s, Merged by: %s\n", pull.Title, pull.User.Login, pull.MergeCommit.Author.Login)
		}
		fmt.Println()
	}
}

func fetchPulls(userName, repoName string, pulls *[]PullRequest, page int) error {
	// Construct Github API URL
	apiUrl := fmt.Sprintf("%s%s&page=%s", baseUrl, strings.ReplaceAll(apiPath, "<username>/<repo_name>", fmt.Sprintf("%s/%s", userName, repoName)), fmt.Sprintf("%d", page))

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 10 * time.Second}

	// Make API request and handle errors
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		fmt.Println("Error creating reuqest:", err)
	}

	// Set the "Accept" header to specify the version of the Github API to use
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Set the "Authorization" header to specify the Bearer ghtoken for Github API
	// autorized use
	ghtoken = os.Getenv("GHTOKEN")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ghtoken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Send the HTTP request and store the response
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
	}

	// Make sure to close the response body when done
	defer resp.Body.Close()

	// Read the response body into a byte slice
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return err
	}

	fetchedPulls := []PullRequest{}
	// Decode API response into slice of pull requests
	err = json.Unmarshal(body, &fetchedPulls)
	if err != nil {
		fmt.Printf("Error decoding API response: %s", err.Error())
		return err
	}
	*pulls = append(*pulls, fetchedPulls...)

	return nil
}

func getMergeCommitAuthor(mergeCommitHash string) string {
	client := &http.Client{Timeout: 10 * time.Second}

	apiUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", os.Args[1], os.Args[2], mergeCommitHash)
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	// Set the "Authorization" header to specify the Bearer ghtoken for Github API
	// autorized use
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ghtoken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var commit struct {
		Commit struct {
			Author struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
	}

	// Read the response body into a byte slice
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		log.Panic(err)
	}

	err = json.Unmarshal(body, &commit)
	if err != nil {
		fmt.Printf("Error decoding commits API response: %s", err.Error())
		os.Exit(1)
	}

	return fmt.Sprintf("%s <%s>", commit.Commit.Author.Name, commit.Commit.Author.Email)
}
